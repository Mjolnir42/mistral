/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/mjolnir42/mistral/internal/mistral"

import (
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

// shutdown indicates that a shutdown has been requested
var shutdown bool

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
	Metrics  *metrics.Registry
	trackID  map[string]*erebos.Transport
	dispatch chan<- *sarama.ProducerMessage
	producer sarama.AsyncProducer
}

// SetUnavailable switches the private package variable to true
func SetUnavailable() {
	unavailable = true
}

// SetShutdown switches the private package variable to true
func SetShutdown() {
	shutdown = true
}

// ackClientRequest updates the API client with the result of
// the producer request
func (m *Mistral) ackClientRequest(trackingID string, err error) {
	if _, ok := m.trackID[trackingID]; !ok {
		logrus.Warnf("Unknown trackingID: %s", trackingID)
		return
	}

	// ack client request
	go func(msg *erebos.Transport, err error) {
		msg.Return <- err
	}(m.trackID[trackingID], err)

	// cleanup request tracking
	delete(m.trackID, trackingID)
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
