/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/mjolnir42/mistral/lib/mistral"

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
	"github.com/mjolnir42/legacy"
)

// Endpoint is the HTTP API endpoint for Mistral
func Endpoint(w http.ResponseWriter, r *http.Request,
	params httprouter.Params) {

	// no new requests are served if the service is
	// considered unavailable
	if unavailable {
		http.Error(w,
			http.StatusText(http.StatusServiceUnavailable),
			http.StatusServiceUnavailable,
		)
		return
	}

	if r.Body == nil {
		http.Error(w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}
	buf, _ := ioutil.ReadAll(r.Body)

	// verify the received data can be parsed
	if err := json.Unmarshal(buf, &legacy.MetricBatch{}); err != nil {
		logrus.Warningf("Rejected unprocessable data from %s: %s",
			r.RemoteAddr, err.Error())

		http.Error(w,
			err.Error(),
			http.StatusUnprocessableEntity,
		)
		return
	}

	// get the hostID from the received data
	hostID, err := legacy.PeekHostID(buf)
	if err != nil {
		logrus.Warningf("Could not get HostID in data from %s: %s",
			r.RemoteAddr, err.Error())

		http.Error(w,
			err.Error(),
			http.StatusUnprocessableEntity,
		)
		return
	} else if hostID == 0 {
		logrus.Warningf("Rejected invalid HostID 0 from %s",
			r.RemoteAddr)

		http.Error(w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}
	ret := make(chan error)
	Handlers[hostID%runtime.NumCPU()].Input <- Transport{
		HostID: hostID,
		Value:  buf,
		Return: ret,
	}
	res := <-ret
	if res != nil {
		logrus.Errorf(
			"Could not write data for HostID %d from %s to Kafka: %s",
			hostID, r.RemoteAddr, res.Error(),
		)

		// set the package variable unavailable to fail /health
		unavailable = true

		http.Error(w,
			http.StatusText(http.StatusServiceUnavailable),
			http.StatusServiceUnavailable,
		)
		return
	}
	logrus.Infof("Processed metric data for HostID %d from %s",
		hostID, r.RemoteAddr)

	w.WriteHeader(http.StatusOK)
	w.Write(nil)
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
