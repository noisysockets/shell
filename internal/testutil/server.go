// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package testutil

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
)

func StartServer(ctx context.Context) (net.Addr, <-chan *websocket.Conn, error) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	wsChan := make(chan *websocket.Conn, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Failed to upgrade connection: %v", err)
			return
		}

		wsChan <- ws

		<-ctx.Done()
	})

	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		_ = server.Serve(lis)
	}()

	return lis.Addr(), wsChan, nil
}
