/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package main // import "github.com/solnx/mistral/cmd/mistral"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
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
	"github.com/mjolnir42/delay"
	"github.com/mjolnir42/erebos"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/solnx/legacy"
	"github.com/solnx/mistral/internal/mistral"
)

var githash, shorthash, builddate, buildtime string
var basicAuthUsername, basicAuthPassword string

func init() {
	// Discard logspam from Zookeeper library
	erebos.DisableZKLogger()

	// set standard logger options
	erebos.SetLogrusOptions()
}

func main() {
	// parse command line flags
	var (
		cliConfPath string
		versionFlag bool
	)
	flag.StringVar(&cliConfPath, `config`, `mistral.conf`,
		`Configuration file location`)
	flag.BoolVar(&versionFlag, `version`, false,
		`Print version information`)
	flag.Parse()

	// only provide version information if --version was specified
	if versionFlag {
		fmt.Fprintln(os.Stderr, `Mistral Metric API`)
		fmt.Fprintf(os.Stderr, "Version  : %s-%s\n", builddate,
			shorthash)
		fmt.Fprintf(os.Stderr, "Git Hash : %s\n", githash)
		fmt.Fprintf(os.Stderr, "Timestamp: %s\n", buildtime)
		os.Exit(0)
	}

	// read runtime configuration
	conf := erebos.Config{}
	if err := conf.FromFile(cliConfPath); err != nil {
		logrus.Fatalf("Could not open configuration: %s", err)
	}

	// setup logfile
	if lfh, err := reopen.NewFileWriter(
		filepath.Join(conf.Log.Path, conf.Log.File),
	); err != nil {
		logrus.Fatalf("Unable to open logfile: %s", err)
	} else {
		conf.Log.FH = lfh
	}
	logrus.SetOutput(conf.Log.FH)
	logrus.Infoln(`Starting MISTRAL...`)

	// signal handler will reopen logfile on USR2 if requested
	if conf.Log.Rotate {
		sigChanLogRotate := make(chan os.Signal, 1)
		signal.Notify(sigChanLogRotate, syscall.SIGUSR2)
		go erebos.Logrotate(sigChanLogRotate, conf)
	}

	// setup signal receiver for graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// this channel is used by the handlers on error
	handlerDeath := make(chan error)

	// setup goroutine waiting policy
	waitdelay := delay.New()

	// setup metrics
	var metricPrefix string
	switch conf.Misc.InstanceName {
	case ``:
		metricPrefix = `/mistral`
	default:
		metricPrefix = fmt.Sprintf("/mistral/%s",
			conf.Misc.InstanceName)
	}
	pfxRegistry := metrics.NewPrefixedRegistry(metricPrefix)
	metrics.NewRegisteredMeter(`/requests`, pfxRegistry)
	metrics.NewRegisteredMeter(`/messages`, pfxRegistry)
	mistral.MtrReg = &pfxRegistry

	ms := legacy.NewMetricSocket(&conf, &pfxRegistry, handlerDeath,
		mistral.FormatMetrics)
	ms.SetDebugFormatter(mistral.DebugFormatMetrics)
	if conf.Misc.ProduceMetrics {
		logrus.Info(`Launched metrics producer socket`)
		waitdelay.Use()
		go func() {
			defer waitdelay.Done()
			ms.Run()
		}()
	}

	// start application handlers
	for i := 0; i < runtime.NumCPU(); i++ {
		h := mistral.Mistral{
			Num: i,
			Input: make(chan *erebos.Transport,
				conf.Mistral.HandlerQueueLength),
			Shutdown: make(chan struct{}),
			Death:    handlerDeath,
			Config:   &conf,
			Metrics:  &pfxRegistry,
		}
		mistral.Handlers[i] = &h
		waitdelay.Use()
		go func() {
			defer waitdelay.Done()
			h.Start()
		}()
		logrus.Infof("Launched Mistral handler #%d", i)
	}

	// assemble listen address
	listenURL := &url.URL{}
	switch conf.Mistral.ListenScheme {
	case ``:
		listenURL.Scheme = `http`
	default:
		listenURL.Scheme = conf.Mistral.ListenScheme
	}
	listenURL.Host = fmt.Sprintf("%s:%s",
		conf.Mistral.ListenAddress,
		conf.Mistral.ListenPort,
	)

	// setup http routes
	router := httprouter.New()
	router.GET(`/health`, mistral.Health)

	// check if authentication is required
	switch conf.Mistral.Authentication {
	case `static_basic_auth`:
		basicAuthUsername = conf.BasicAuth.Username
		basicAuthPassword = conf.BasicAuth.Password
		router.POST(conf.Mistral.EndpointPath, BasicAuth(mistral.Endpoint))
	default:
		router.POST(conf.Mistral.EndpointPath, mistral.Endpoint)
	}

	// setup HTTPserver
	srv := &http.Server{
		Addr:    listenURL.Host,
		Handler: router,
	}

	// setup TLS configuration if required
	if listenURL.Scheme == `https` {
		srv.TLSConfig = &tls.Config{
			PreferServerCipherSuites: true,
		}

		// allow to lower the minimum TLS protocol version
		switch conf.TLS.MinVersion {
		case `TLS1.0`:
			srv.TLSConfig.MinVersion = tls.VersionTLS10
		case `TLS1.1`:
			srv.TLSConfig.MinVersion = tls.VersionTLS11
		case `TLS1.2`:
			srv.TLSConfig.MinVersion = tls.VersionTLS12
		default:
			srv.TLSConfig.MinVersion = tls.VersionTLS12
		}

		// allow to lower the maximum TLS protocol version
		switch conf.TLS.MaxVersion {
		case `TLS1.0`:
			srv.TLSConfig.MaxVersion = tls.VersionTLS10
		case `TLS1.1`:
			srv.TLSConfig.MaxVersion = tls.VersionTLS11
		case `TLS1.2`:
			srv.TLSConfig.MaxVersion = tls.VersionTLS12
		default:
		}

		// allow to limit offered cipher suites
		switch conf.TLS.Ciphers {
		case `strict`:
			srv.TLSConfig.CipherSuites = []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			}
		}

		// load configured certificates
		srv.TLSConfig.Certificates = make([]tls.Certificate, 0, len(conf.TLS.CertificateChains))
		for i := range conf.TLS.CertificateChains {
			cert, err := tls.LoadX509KeyPair(
				conf.TLS.CertificateChains[i].ChainFile,
				conf.TLS.CertificateChains[i].KeyFile,
			)
			if err != nil {
				logrus.Fatalf("Failed to load TLS certificate: %s", err)
			}
			srv.TLSConfig.Certificates = append(
				srv.TLSConfig.Certificates,
				cert,
			)
		}
		srv.TLSConfig.BuildNameToCertificate()

		// load configured root CAs
		srv.TLSConfig.RootCAs = x509.NewCertPool()
		for i := range conf.TLS.RootCAs {
			rootPEM, err := ioutil.ReadFile(conf.TLS.RootCAs[i])
			if err != nil {
				logrus.Fatalf("Failed to read RootCA from file: %s, %s",
					conf.TLS.RootCAs[i], err,
				)
			}
			if !srv.TLSConfig.RootCAs.AppendCertsFromPEM(rootPEM) {
				logrus.Fatalf("Failed to load RootCA from %s", conf.TLS.RootCAs[i])
			}
		}
	}

	// delay a bit, then check for early startup errors by the
	// application handlers. Skip starting the HTTP server if a fatal
	// error has already occured
	<-time.After(250 * time.Millisecond)
	select {
	case err := <-handlerDeath:
		// early error occured, put it back to initiate the shutdown
		// sequence
		logrus.Errorln(`Early startup error detected - HTTP server startup will be skipped`)
		waitdelay.Use()
		go func(e error) {
			defer waitdelay.Done()
			handlerDeath <- e
		}(err)
		goto skipHTTP
	default:
	}

	// start http || https server
	waitdelay.Use()
	go func() {
		defer waitdelay.Done()
		var err error
		logrus.Infoln("Starting HTTP/HTTPS server in mode: %s",
			listenURL.Scheme)
		switch listenURL.Scheme {
		case `http`:
			err = srv.ListenAndServe()
		case `https`:
			// certificates are already configured in srv.TLSConfig
			err = srv.ListenAndServeTLS(``, ``)
		}
		if err != nil {
			handlerDeath <- err
		}
	}()

	// the main loop
skipHTTP:
	fault := false
	shutdown := false
	startupDelay := time.NewTimer(time.Second)
runloop:
	for {
		select {
		case <-startupDelay.C:
			// no error was reported during early startup, this instance
			// is open for business
			mistral.StartupComplete()
		case err := <-ms.Errors:
			logrus.Errorf("Socket error: %s", err.Error())
		case <-c:
			logrus.Infoln(`Received shutdown signal`)
			// switch the application to shutdown which will cause
			// healthchecks to fail.
			mistral.SetShutdown()
			shutdown = true
			break runloop
		case err := <-handlerDeath:
			logrus.Errorf("Handler died: %s", err.Error())
			// switch the application to unavailable which will cause
			// healthchecks to fail. The shutdown race against the
			// watchdog begins.
			// All new http connections will now also fail.
			mistral.SetUnavailable()
			fault = true
			// drain startupDelay timer if applicable
			if !startupDelay.Stop() {
				<-startupDelay.C
			}
			break runloop
		}
	}
	logrus.Infoln(`main.runloop exited, shutdown sequence running`)

	if shutdown {
		logrus.Infoln(`Graceful shutdown waiting 95 seconds with failed health check`)
		// give the loadbalancer time to pick up the failing health
		// check and remove this instance from service
		<-time.After(time.Second * 95)
	}

	// close all handlers
	close(ms.Shutdown)
	for i := range mistral.Handlers {
		close(mistral.Handlers[i].ShutdownChannel())
		close(mistral.Handlers[i].InputChannel())
	}
	logrus.Info(`Handler channels closed`)

	// stop http server
	ctx, cancel := context.WithTimeout(
		context.Background(), 5*time.Second)
	defer cancel()
	logrus.Info(`Stopping HTTP server`)
	if err := srv.Shutdown(ctx); err != nil {
		logrus.Warnf("HTTP shutdown error: %s", err.Error())
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
	logrus.Infoln(`Drained all channels`)

	// give goroutines that were blocked on handlerDeath channel
	// a chance to exit
	logrus.Infoln(`Waiting for go-routines to exit`)
	waitdelay.Wait()

	logrus.Infoln(`MISTRAL shutdown complete`)
	if fault {
		os.Exit(1)
	}
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
