/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
	"github.com/mjolnir42/legacy"
)

// Endpoint is the HTTP API endpoint for Mistral
func Endpoint(w http.ResponseWriter, r *http.Request,
	params httprouter.Params) {

	if r.Body == nil {
		http.Error(w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}
	buf, _ := ioutil.ReadAll(r.Body)

	// verify the received data can be parsed
	if err := json.Unmarshal(buf, legacy.MetricBatch{}); err != nil {
		log.Warningf("Rejected unprocessable data from %s: %s",
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
		log.Warningf("Could not get HostID in data from %s: %s",
			r.RemoteAddr, err.Error())

		http.Error(w,
			err.Error(),
			http.StatusInternalServerError,
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
		log.Errorf(
			"Could not write data for HostID %d from %s to Kafka: %s",
			hostID, r.RemoteAddr, err.Error(),
		)

		http.Error(w,
			res.Error(),
			http.StatusInternalServerError,
		)
		return
	}
	log.Infof("Processed metric data for HostID %d from %s\n",
		hostID, r.RemoteAddr)

	w.WriteHeader(http.StatusOK)
	w.Write(nil)
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
