/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/mjolnir42/mistral/internal/mistral"

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
	"github.com/mjolnir42/erebos"
	"github.com/mjolnir42/legacy"
	metrics "github.com/rcrowley/go-metrics"
)

// Endpoint is the HTTP API endpoint for Mistral
func Endpoint(w http.ResponseWriter, r *http.Request,
	params httprouter.Params) {

	// count all requests, declined or accepted
	if MtrReg != nil {
		mtr := metrics.GetOrRegisterMeter(`/requests`, *MtrReg)
		mtr.Mark(1)
	}

	// no new requests are served if the service is
	// considered unavailable
	if unavailable {
		logrus.Infof("Unavailable - request from %s rejected", r.RemoteAddr)
		http.Error(w,
			http.StatusText(http.StatusServiceUnavailable),
			http.StatusServiceUnavailable,
		)
		return
	}

	if r.Body == nil {
		logrus.Warningf("Rejected empty request body from %s", r.RemoteAddr)
		http.Error(w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}
	buf, _ := ioutil.ReadAll(r.Body)

	// verify the received data can be parsed
	batch := &legacy.MetricBatch{}
	if err := json.Unmarshal(buf, batch); err != nil {
		logrus.Warningf(
			"json.Unmarshal: rejected unprocessable data from %s: %s",
			r.RemoteAddr, err.Error())

		http.Error(w,
			err.Error(),
			http.StatusUnprocessableEntity,
		)
		return
	}

	// get the hostID from the received data
	hostID := batch.HostID
	if hostID == 0 {
		logrus.Warningf("Rejected invalid HostID 0 from %s",
			r.RemoteAddr)

		http.Error(w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}

	// encode back to JSON, this Unmarshal/Marshal step fixes and
	// converts some broken metrics
	var err error
	var fixed []byte
	if fixed, err = json.Marshal(batch); err != nil {
		logrus.Errorf(
			"json.Marshal: rejected unprocessable data from %s: %s",
			r.RemoteAddr, err.Error())

		http.Error(w,
			err.Error(),
			http.StatusUnprocessableEntity,
		)
		return
	}

	// send data to application handler for kafka production
	ret := make(chan error)
	Dispatch(erebos.Transport{
		HostID: hostID,
		Value:  fixed,
		Return: ret,
	})

	// wait for kafka result
	res := <-ret
	if res != nil {
		logrus.Errorf(
			"Could not write data for HostID %d from %s to Kafka: %s",
			hostID, r.RemoteAddr, res.Error(),
		)

		http.Error(w,
			http.StatusText(http.StatusBadGateway),
			http.StatusBadGateway,
		)
		return
	}
	logrus.Infof("Processed metric data for HostID %d from %s",
		hostID, r.RemoteAddr)

	w.WriteHeader(http.StatusOK)
	w.Write(nil)
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
