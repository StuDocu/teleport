// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"syscall"

	"github.com/gravitational/teleport/lib/terminal"
	"github.com/gravitational/teleport/tool/tshd/config"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var (
	logFormat = flag.String("log_format", "", "Log format to use (json or text)")
	logLevel  = flag.String("log_level", "", "Log level to use")

	addr       = flag.String("addr", "tcp://localhost:", "Bind address for the Terminal server")
	certFile   = flag.String("cert_file", "", "Cert file (or inline PEM) for the Terminal server. Enables TLS.")
	certKey    = flag.String("cert_key", "", "Key file (or inline PEM) for the Terminal server. Enables TLS.")
	clientCAs  = flag.String("client_cas", "", "Client CA certificate (or inline PEM) for the Terminal server. Enables mTLS.")
	stdin      = flag.Bool("stdin", false, "Read server configuration from stdin")
	configPath = flag.String("config", "", "config file")
)

func main() {
	flag.Parse()
	cfg, err := config.New(*configPath)
	if err != nil {
		log.Fatal(trace.Wrap(err))
	}

	if err := json.NewDecoder(os.Stdout).Decode(&cfg); err != nil {
		log.Fatal(trace.Wrap(err))
	}

	configureLogging(cfg)

	if err := run(); err != nil {
		log.Fatal(trace.Wrap(err))
	}
}

func configureLogging(cfg config.Config) {
	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
		log.SetFormatter(&trace.TextFormatter{})
	} else {
		// Production
		logrus.SetFormatter(&trace.JSONFormatter{})
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var trustedCAs []string
	if *clientCAs != "" {
		trustedCAs = []string{*clientCAs}
	}
	server, err := terminal.Start(ctx, terminal.ServerOpts{
		Addr:            *addr,
		CertFile:        *certFile,
		KeyFile:         *certKey,
		ClientCAs:       trustedCAs,
		ReadFromInput:   *stdin,
		ConfigInput:     os.Stdin,
		ConfigOutput:    os.Stdout,
		ShutdownSignals: []os.Signal{os.Interrupt, syscall.SIGTERM},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("tshd running at %v", server.Addr)
	return <-server.C
}
