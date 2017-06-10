/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package main // import "github.com/mjolnir42/mistral/cmd/mistral"

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/client9/reopen"
	"github.com/julienschmidt/httprouter"
	"github.com/mjolnir42/erebos"
	"github.com/mjolnir42/mistral/lib/mistral"
	metrics "github.com/rcrowley/go-metrics"
)

func init() {
	// Discard logspam from Zookeeper library
	erebos.DisableZKLogger()

	// set standard logger options
	erebos.SetLogrusOptions()
}

func main() {
	// parse command line flags
	var cliConfPath string
	flag.StringVar(&cliConfPath, `config`, `mistral.conf`,
		`Configuration file location`)
	flag.Parse()

	// read runtime configuration
	miConf := erebos.Config{}
	if err := miConf.FromFile(cliConfPath); err != nil {
		logrus.Fatalf("Could not open configuration: %s", err)
	}

	// setup logfile
	if lfh, err := reopen.NewFileWriter(
		filepath.Join(miConf.Log.Path, miConf.Log.File),
	); err != nil {
		logrus.Fatalf("Unable to open logfile: %s", err)
	} else {
		miConf.Log.FH = lfh
	}
	logrus.SetOutput(miConf.Log.FH)
	logrus.Infoln(`Starting MISTRAL...`)

	// signal handler will reopen logfile on USR2 if requested
	if miConf.Log.Rotate {
		sigChanLogRotate := make(chan os.Signal, 1)
		signal.Notify(sigChanLogRotate, syscall.SIGUSR2)
		go erebos.Logrotate(sigChanLogRotate, miConf)
	}

	// setup metrics
	pfxRegistry := metrics.NewPrefixedRegistry(`mistral`)
	metrics.NewRegisteredMeter(`.messages`, pfxRegistry)

	go func(r *metrics.Registry) {
		beat := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-beat.C:
				(*r).Each(printMetrics)
			}
		}
	}(&pfxRegistry)

	// start one handler per CPU
	for i := 0; i < runtime.NumCPU(); i++ {
		h := mistral.Mistral{
			Num: i,
			Input: make(chan erebos.Transport,
				miConf.Mistral.HandlerQueueLength),
			Config:  &miConf,
			Metrics: &pfxRegistry,
		}
		mistral.Handlers[i] = &h
		go h.Start()
		logrus.Infof("Launched Mistral handler #%d", i)
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
	logrus.Fatal(http.ListenAndServe(listenURL.Host, router))
}

func printMetrics(metric string, v interface{}) {
	switch v.(type) {
	case *metrics.StandardMeter:
		value := v.(*metrics.StandardMeter)
		fmt.Fprintf(os.Stderr, "%s.avg.rate.1min: %f\n", metric, value.Rate1())
	}
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
