/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/mjolnir42/mistral/lib/mistral"

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Shopify/sarama"
	"github.com/mjolnir42/erebos"
	metrics "github.com/rcrowley/go-metrics"
	kazoo "github.com/wvanbergen/kazoo-go"
)

// Handlers must be set before Mistral.Start is called for
// the first time. It is used by Endpoint to look up the running
// Mistral handlers
var Handlers map[int]erebos.Handler

// unavailable indicates that producing to Kafka returned errors
var unavailable bool

func init() {
	Handlers = make(map[int]erebos.Handler)
}

// Mistral produces messages received via its HTTP handler to Kafka
type Mistral struct {
	Num      int
	Input    chan Transport
	Config   *erebos.Config
	producer sarama.SyncProducer
	Metrics  *metrics.Registry
}

// Transport is a small wrapper between the HTTP handlers and Mistral
// in order to return errors back
type Transport struct {
	HostID int
	Value  []byte
	Return chan error
}

// Start sets up a Mistral application handler
func (m *Mistral) Start() {
	if len(Handlers) == 0 {
		panic(`Incorrectly set handlers`)
	}

	kz, err := kazoo.NewKazooFromConnectionString(
		m.Config.Zookeeper.Connect, nil)
	if err != nil {
		panic(err)
	}
	brokers, err := kz.BrokerList()
	if err != nil {
		kz.Close()
		panic(err)
	}
	kz.Close()

	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	config := sarama.NewConfig()
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
		panic(err)
	}
	m.run()
}

// run is the event loop for Mistral
func (m *Mistral) run() {
	mtr := metrics.GetOrRegisterMeter(`.messages`, *m.Metrics)
	for {
		select {
		case msg := <-m.Input:
			m.process(&msg)
			mtr.Mark(1)
		}
	}
	// currently unreachable until graceful shutdown is supported
	//m.producer.Close()
}

// process sends the received message to Kafka
func (m *Mistral) process(msg *Transport) {
	_, _, err := m.producer.SendMessage(&sarama.ProducerMessage{
		Topic: m.Config.Kafka.ProducerTopic,
		Key: sarama.StringEncoder(
			strconv.Itoa(int(msg.HostID)),
		),
		Value: sarama.ByteEncoder(msg.Value),
	})
	msg.Return <- err
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
