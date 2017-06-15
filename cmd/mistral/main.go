/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package main // import "github.com/mjolnir42/mistral/cmd/mistral"

import (
	"context"
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
	"github.com/mjolnir42/legacy"
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

	// setup signal receiver for graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// this channel is used by the handlers on error
	handlerDeath := make(chan error)

	// setup metrics
	pfxRegistry := metrics.NewPrefixedRegistry(`mistral`)
	metrics.NewRegisteredMeter(`.messages`, pfxRegistry)
	mistral.MtrReg = &pfxRegistry

	ms := legacy.NewMetricSocket(&miConf, &pfxRegistry, handlerDeath, mistral.FormatMetrics)
	if miConf.Misc.ProduceMetrics {
		logrus.Info(`Launched metrics producer socket`)
		go ms.Run()
	}

	// start application handlers
	for i := 0; i < runtime.NumCPU(); i++ {
		h := mistral.Mistral{
			Num: i,
			Input: make(chan *erebos.Transport,
				miConf.Mistral.HandlerQueueLength),
			Shutdown: make(chan struct{}),
			Death:    handlerDeath,
			Config:   &miConf,
			Metrics:  &pfxRegistry,
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
	srv := &http.Server{
		Addr:    listenURL.Host,
		Handler: router,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			handlerDeath <- err
		}
	}()

	// the main loop
	fault := false
runloop:
	for {
		select {
		case err := <-ms.Errors:
			logrus.Errorf("Socket error: %s", err.Error())
		case <-c:
			logrus.Infoln(`Received shutdown signal`)
			break runloop
		case err := <-handlerDeath:
			logrus.Errorf("Handler died: %s", err.Error())
			fault = true
			break runloop
		}
	}
	// switch the application to unavailable which will cause
	// healthchecks to fail. The shutdown race against the watchdog
	// begins. All new http connections will now also fail.
	mistral.SetUnavailable()

	// close all handlers
	close(ms.Shutdown)
	for i := range mistral.Handlers {
		close(mistral.Handlers[i].ShutdownChannel())
		close(mistral.Handlers[i].InputChannel())
	}

	// read all additional handler errors if required
drainloop:
	for {
		select {
		case err := <-ms.Errors:
			logrus.Errorf("Socket error: %s", err.Error())
		case err := <-handlerDeath:
			logrus.Errorf("Handler error: %s", err.Error())
		case <-time.After(time.Millisecond * 10):
			break drainloop
		}
	}

	// give goroutines that were blocked on handlerDeath channel
	// a chance to exit
	<-time.After(time.Millisecond * 10)

	// stop http server
	ctx, cancel := context.WithTimeout(
		context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logrus.Warnf("HTTP shutdown error: %s", err.Error())
	}
	logrus.Infoln(`MISTRAL shutdown complete`)
	if fault {
		os.Exit(1)
	}
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
