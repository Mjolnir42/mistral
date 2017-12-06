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

	"github.com/Sirupsen/logrus"
	metrics "github.com/rcrowley/go-metrics"
)

// run is the event loop for Mistral
func (m *Mistral) run() {
	mtr := metrics.GetOrRegisterMeter(
		`/messages`,
		*m.Metrics,
	)

	// required during shutdown
	inputEmpty := false
	errorEmpty := false
	successEmpty := false
	producerClosed := false

runloop:
	for {
		select {
		case <-m.Shutdown:
			goto drainloop
		case msg := <-m.producer.Errors():
			trackingID := msg.Msg.Metadata.(string)
			err := msg.Err
			m.ackClientRequest(trackingID, err)
			mtr.Mark(1)
			logrus.Errorf("Producer error: %s", err.Error())
		case msg := <-m.producer.Successes():
			trackingID := msg.Metadata.(string)
			m.ackClientRequest(trackingID, nil)
			mtr.Mark(1)
		case msg := <-m.Input:
			if msg == nil {
				// read from closed Input channel before closed
				// Shutdown channel
				continue runloop
			}
			m.process(msg)
		}
	}
	m.producer.Close()
	return

	// drain the input channel
drainloop:
	for {
		select {
		case msg := <-m.Input:
			if msg == nil {
				inputEmpty = true

				if !producerClosed {
					m.producer.Close()
					producerClosed = true
				}

				//channels are closed
				if inputEmpty && errorEmpty && successEmpty {
					break drainloop
				}
				continue drainloop
			}
			m.process(msg)
		case msg := <-m.producer.Errors():
			if msg == nil {
				errorEmpty = true

				// channels are closed
				if inputEmpty && errorEmpty && successEmpty {
					break drainloop
				}
				continue drainloop
			}
			trackingID := msg.Msg.Metadata.(string)
			err := msg.Err
			m.ackClientRequest(trackingID, err)
			mtr.Mark(1)
			logrus.Errorf("Producer error: %s", err.Error())
		case msg := <-m.producer.Successes():
			if msg == nil {
				successEmpty = true

				// channels are closed
				if inputEmpty && errorEmpty && successEmpty {
					break drainloop
				}
				continue drainloop
			}
			trackingID := msg.Metadata.(string)
			m.ackClientRequest(trackingID, nil)
			mtr.Mark(1)
		}
	}
	m.delay.Wait()
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
