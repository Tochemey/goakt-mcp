// MIT License
//
// Copyright (c) 2026 GoAkt Team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package main is the service entrypoint for the goakt-mcp gateway.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/internal/runtime/actor"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
)

// envConfigPath is the environment variable consulted for the config file path
// when the -config flag is not provided.
const envConfigPath = "GOAKT_MCP_CONFIG"

func main() {
	if err := run(); err != nil {
		goaktlog.DefaultLogger.Errorf("%v", err)
		os.Exit(1)
	}
}

// run is the top-level application lifecycle: load config, start the gateway,
// wait for a signal, and shut down gracefully. It returns a non-nil error on
// any fatal condition.
func run() error {
	conf, err := loadConfig()
	if err != nil {
		return err
	}

	logger := config.NewLogger(conf.LogLevel)

	gw, err := actor.New(conf, logger)
	if err != nil {
		return fmt.Errorf("create gateway: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := gw.Start(ctx); err != nil {
		return fmt.Errorf("start gateway: %w", err)
	}

	logger.Info("gateway started; press Ctrl+C to stop")
	awaitShutdownSignal()
	logger.Info("shutting down...")

	stopCtx, stopCancel := context.WithTimeout(context.Background(), conf.Runtime.ShutdownTimeout)
	defer stopCancel()

	if err := gw.Stop(stopCtx); err != nil {
		logger.Warnf("stop gateway: %v", err)
	}

	return nil
}

// loadConfig resolves the configuration from, in order of priority:
//  1. The path given by the -config flag.
//  2. The path in the GOAKT_MCP_CONFIG environment variable.
//  3. Built-in defaults when neither source is set.
func loadConfig() (config.Config, error) {
	configPath := flag.String("config", "", "path to YAML config file (or set "+envConfigPath+")")
	flag.Parse()

	if *configPath == "" {
		*configPath = os.Getenv(envConfigPath)
	}

	if *configPath != "" {
		loaded, err := config.LoadFile(*configPath)
		if err != nil {
			return config.Config{}, fmt.Errorf("load config: %w", err)
		}
		return *loaded, nil
	}

	var cfg config.Config
	config.ApplyDefaults(&cfg)
	return cfg, nil
}

// awaitShutdownSignal blocks until SIGINT or SIGTERM is received.
func awaitShutdownSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	<-sigCh
}
