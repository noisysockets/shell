/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

// Package main implements an insecure WebSocket server that can be used to
// create a shell session. This package is intended for testing purposes only!
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/noisysockets/shell"
	"github.com/rs/cors"
)

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost",
			"http://localhost:*",
			"http://127.0.0.1:*",
			"http://[::1]:*",
		},
	})

	upgrader := websocket.Upgrader{
		CheckOrigin: corsHandler.OriginAllowed,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Noisy Sockets Shell development server, If this is on the internet, please immediately report it!")
	})

	mux.HandleFunc("/shell/ws", func(w http.ResponseWriter, r *http.Request) {
		logger := logger.With(slog.Any("remote", r.RemoteAddr))

		logger.Info("Handling connection")

		if !netip.MustParseAddrPort(r.RemoteAddr).Addr().IsLoopback() {
			logger.Warn("Only local connections are allowed")

			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "Forbidden")
			return
		}

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("Failed to upgrade connection", slog.Any("error", err))
			return
		}

		h, err := shell.NewHandler(r.Context(), logger, ws)
		if err != nil {
			logger.Error("Failed to create shell handler", slog.Any("error", err))

			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Failed to create shell handler: %v", err)
			return
		}
		defer h.Close()

		if err := h.Wait(); err != nil {
			logger.Error("Shell handler failed", slog.Any("error", err))

			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Shell handler failed: %v", err)
			return
		}

		logger.Info("Shell handler completed")
	})

	srv := &http.Server{
		Handler: mux,
	}

	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", "localhost:8080")
	if err != nil {
		logger.Error("Failed to listen", slog.Any("error", err))
		os.Exit(1)
	}

	// Capture SIGINT and SIGTERM signals to gracefully shutdown the server.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sig

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		_ = srv.Shutdown(ctx)
	}()

	logger.Info("Listening on", slog.Any("address", lis.Addr()))

	if err := srv.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("Failed to start server", slog.Any("error", err))
		os.Exit(1)
	}
}
