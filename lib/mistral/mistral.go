/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/mjolnir42/mistral/lib/mistral"

import (
	"strconv"

	"github.com/Shopify/sarama"
	"github.com/mjolnir42/erebos"
	metrics "github.com/rcrowley/go-metrics"
)

// Handlers must be set before Mistral.Start is called for
// the first time. It is used by Endpoint to look up the running
// Mistral handlers
var Handlers map[int]erebos.Handler

// MtrReg is the go-metrics Registry reference for the HTTP handler functions
var MtrReg *metrics.Registry

// unavailable indicates that producing to Kafka returned errors
var unavailable bool

func init() {
	Handlers = make(map[int]erebos.Handler)
}

// Mistral produces messages received via its HTTP handler to Kafka
type Mistral struct {
	Num      int
	Input    chan *erebos.Transport
	Shutdown chan struct{}
	Death    chan error
	Config   *erebos.Config
	producer sarama.SyncProducer
	Metrics  *metrics.Registry
}

// run is the event loop for Mistral
func (m *Mistral) run() {
	mtr := metrics.GetOrRegisterMeter(`/messages`, *m.Metrics)
runloop:
	for {
		select {
		case <-m.Shutdown:
			break runloop
		case msg := <-m.Input:
			if msg == nil {
				// read from closed Input channel before closed
				// Shutdown channel
				continue runloop
			}
			m.process(msg)
			mtr.Mark(1)
		}
	}

	// drain the input channel
drainloop:
	for {
		select {
		case msg := <-m.Input:
			if msg == nil {
				// channel is closed
				break drainloop
			}
			m.process(msg)
		}
	}
	m.producer.Close()
}

// process sends the received message to Kafka
func (m *Mistral) process(msg *erebos.Transport) {
	_, _, err := m.producer.SendMessage(&sarama.ProducerMessage{
		Topic: m.Config.Kafka.ProducerTopic,
		Key: sarama.StringEncoder(
			strconv.Itoa(int(msg.HostID)),
		),
		Value: sarama.ByteEncoder(msg.Value),
	})
	msg.Return <- err
}

// SetUnavailable switches the private package variable to true
func SetUnavailable() {
	unavailable = true
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
