/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/solnx/mistral/internal/mistral"

import (
	"time"

	"github.com/Sirupsen/logrus"
)

// init starts the watchdog
func init() {
	go watchdog()
}

// watchdog checks if the service is declared unavailable and kills it
// 70 seconds later
func watchdog() {
	tock := time.NewTicker(3 * time.Second)
	for {
		select {
		case <-tock.C:
			if unavailable {
				tock.Stop()
				// allow the loadbalancer to pick up the failing health
				time.Sleep(70 * time.Second)
				logrus.Fatalln(`Watchdog terminated Mistral: service unavailable`)
			}
		}
	}
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
