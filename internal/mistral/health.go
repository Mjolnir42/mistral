/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/mjolnir42/mistral/internal/mistral"

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	metrics "github.com/rcrowley/go-metrics"
)

// Health is the HTTP API healthcheck for Mistral. It returns 204
// if the service is healthy or 503 if the service experienced
// errors
func Health(w http.ResponseWriter, r *http.Request,
	_ httprouter.Params) {

	// count /health requests against received http requests
	if MtrReg != nil {
		mtr := metrics.GetOrRegisterMeter(`/requests`, *MtrReg)
		mtr.Mark(1)
	}

	if unavailable || shutdown {

		http.Error(w,
			http.StatusText(http.StatusServiceUnavailable),
			http.StatusServiceUnavailable,
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Write(nil)
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
