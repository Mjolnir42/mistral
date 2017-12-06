/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/mjolnir42/mistral/lib/mistral"

import (
	metrics "github.com/rcrowley/go-metrics"
)

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

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
