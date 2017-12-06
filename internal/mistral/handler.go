/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/mjolnir42/mistral/internal/mistral"

import (
	"fmt"
	"os"
	"time"

	"github.com/Shopify/sarama"
	"github.com/mjolnir42/erebos"
	kazoo "github.com/wvanbergen/kazoo-go"
)

// Implementation of the erebos.Handler interface

// Start sets up a Mistral application handler
func (m *Mistral) Start() {
	if len(Handlers) == 0 {
		m.Death <- fmt.Errorf(`Incorrectly set handlers`)
		<-m.Shutdown
		return
	}

	kz, err := kazoo.NewKazooFromConnectionString(
		m.Config.Zookeeper.Connect, nil)
	if err != nil {
		m.Death <- err
		<-m.Shutdown
		return
	}
	brokers, err := kz.BrokerList()
	if err != nil {
		kz.Close()
		m.Death <- err
		<-m.Shutdown
		return
	}
	kz.Close()

	host, err := os.Hostname()
	if err != nil {
		m.Death <- err
		<-m.Shutdown
		return
	}

	config := sarama.NewConfig()
	// required for sync producers
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	// set producer transport keepalive
	switch m.Config.Kafka.Keepalive {
	case 0:
		config.Net.KeepAlive = 3 * time.Second
	default:
		config.Net.KeepAlive = time.Duration(m.Config.Kafka.Keepalive) * time.Millisecond
	}
	// set our required persistence confidence for producing
	switch m.Config.Kafka.ProducerResponseStrategy {
	case `NoResponse`:
		config.Producer.RequiredAcks = sarama.NoResponse
	case `WaitForLocal`:
		config.Producer.RequiredAcks = sarama.WaitForLocal
	case `WaitForAll`:
		config.Producer.RequiredAcks = sarama.WaitForAll
	default:
		config.Producer.RequiredAcks = sarama.WaitForLocal
	}
	// set how often to retry producing
	switch m.Config.Kafka.ProducerRetry {
	case 0:
		config.Producer.Retry.Max = 3
	default:
		config.Producer.Retry.Max = m.Config.Kafka.ProducerRetry
	}
	config.Producer.Partitioner = sarama.NewHashPartitioner
	config.ClientID = fmt.Sprintf("mistral.%s", host)

	m.producer, err = sarama.NewSyncProducer(brokers, config)
	if err != nil {
		m.Death <- err
		<-m.Shutdown
		return
	}
	m.run()
}

// InputChannel returns the data input channel
func (m *Mistral) InputChannel() chan *erebos.Transport {
	return m.Input
}

// ShutdownChannel returns the shutdown signal channel
func (m *Mistral) ShutdownChannel() chan struct{} {
	return m.Shutdown
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
