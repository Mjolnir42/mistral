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
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/client9/reopen"
	"github.com/julienschmidt/httprouter"
	"github.com/mjolnir42/erebos"
	"github.com/mjolnir42/mistral/lib/mistral"
)

func main() {

	// read runtime configuration
	miConf := erebos.Config{}
	if err := miConf.FromFile(`mistral.conf`); err != nil {
		log.Fatalf("Could not open configuration: %s", err)
	}

	// setup logfile
	if lfh, err := reopen.NewFileWriter(
		filepath.Join(miConf.Log.Path, miConf.Log.File),
	); err != nil {
		log.Fatalf("Unable to open logfile: %s", err)
	} else {
		miConf.Log.FH = lfh
	}
	log.SetOutput(miConf.Log.FH)

	// signal handler will reopen logfile on USR2 if requested
	if miConf.Log.Rotate {
		sigChanLogRotate := make(chan os.Signal, 1)
		signal.Notify(sigChanLogRotate, syscall.SIGUSR2)
		go erebos.Logrotate(sigChanLogRotate, miConf)
	}

	// register handlers with package, used by the HTTP endpoint
	// to look up the correct handler
	handlers := make(map[int]mistral.Mistral)
	mistral.Handlers = handlers

	// start one handler per CPU
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

	// assemble listen address
	listenURL := &url.URL{}
	listenURL.Scheme = `http`
	listenURL.Host = fmt.Sprintf("%s:%s",
		miConf.Mistral.ListenAddress,
		miConf.Mistral.ListenPort,
	)

	// setup http routes
	router := httprouter.New()
	router.POST(miConf.Mistral.EndpointPath, mistral.Endpoint)
	router.GET(`/health`, mistral.Health)

	// start HTTPserver
	log.Fatal(http.ListenAndServe(listenURL.Host, router))
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
