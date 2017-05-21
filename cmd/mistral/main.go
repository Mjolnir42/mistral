/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package main // import "github.com/mjolnir42/mistral/cmd/mistral"

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"runtime"

	"github.com/julienschmidt/httprouter"
	"github.com/mjolnir42/erebos"
	"github.com/mjolnir42/mistral/lib/mistral"
)

func main() {
	miConf := erebos.Config{}
	if err := miConf.FromFile(`mistral.conf`); err != nil {
		log.Fatalln(err)
	}

	handlers := make(map[int]mistral.Mistral)
	mistral.Handlers = handlers

	for i := 0; i < runtime.NumCPU(); i++ {
		h := mistral.Mistral{
			Num: i,
			Input: make(chan mistral.Transport,
				miConf.Mistral.HandlerQueueLength),
			Config: &miConf,
		}
		handlers[i] = h
		go h.Start()
	}

	listenURL := &url.URL{}
	listenURL.Scheme = `http`
	listenURL.Host = fmt.Sprintf("%s:%s",
		miConf.Mistral.ListenAddress,
		miConf.Mistral.ListenPort,
	)

	router := httprouter.New()

	router.POST(miConf.Mistral.EndpointPath, mistral.Endpoint)
	log.Fatal(http.ListenAndServe(listenURL.Host, router))
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
