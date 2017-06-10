/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/mjolnir42/mistral/lib/mistral"

import (
	"runtime"

	"github.com/mjolnir42/erebos"
)

// Dispatch implements erebos.Dispatcher
func Dispatch(msg erebos.Transport) {
	// send all messages with the same HostID to the same handler
	// to keep the ordering intact

	Handlers[msg.HostID%runtime.NumCPU()].InputChannel() <- msg
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
