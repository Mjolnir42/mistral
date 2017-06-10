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

// unavailable indicates that producing to Kafka returned errors
var unavailable bool

func init() {
	Handlers = make(map[int]erebos.Handler)
}

// Mistral produces messages received via its HTTP handler to Kafka
type Mistral struct {
	Num      int
	Input    chan erebos.Transport
	Shutdown chan struct{}
	Config   *erebos.Config
	producer sarama.SyncProducer
	Metrics  *metrics.Registry
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

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
