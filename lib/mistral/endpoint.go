/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral

import (
	"io/ioutil"
	"net/http"
	"runtime"

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

	hostID, err := legacy.PeekHostID(buf)
	if err != nil {
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
		http.Error(w,
			res.Error(),
			http.StatusInternalServerError,
		)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(nil)
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
