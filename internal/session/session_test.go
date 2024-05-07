// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package session_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/noisysockets/shell/internal/message"
	"github.com/noisysockets/shell/internal/message/v1alpha1"
	"github.com/noisysockets/shell/internal/session"
	"github.com/noisysockets/shell/internal/testutil"

	"github.com/gorilla/websocket"
	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession(t *testing.T) {
	logger := slogt.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	addr, serverWsChan, err := testutil.StartServer(ctx)
	require.NoError(t, err)

	// Connect to the server.
	clientWs, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/ws", addr), nil)
	require.NoError(t, err)

	// Create a new session for the client.
	clientSession, err := session.NewSession(ctx, logger, clientWs, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = clientSession.Close()
	})

	// Get the server's websocket connection.
	serverWs := <-serverWsChan

	receivedOpenMessages := make(chan message.Message, 1)
	handleTerminalOpenRequest := func(msg message.Message) (bool, error) {
		receivedOpenMessages <- msg.(*v1alpha1.TerminalOpenRequest)

		return true, nil
	}

	// Create a new session for the server.
	serverSession, err := session.NewSession(ctx, logger, serverWs, &session.Config{
		MessageHandlers: map[string]session.MessageHandler{message.Type(new(v1alpha1.TerminalOpenRequest)): handleTerminalOpenRequest},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = serverSession.Close()
	})

	t.Run("Data Read/Write", func(t *testing.T) {
		// Generate 10MB of random data.
		data := make([]byte, 10*1024*1024)
		_, err = rand.Read(data)
		require.NoError(t, err)

		// Test sending data from the client to the server.
		go func() {
			n, err := clientSession.Write(data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write data: %v", err)
			}

			if n != len(data) {
				fmt.Fprintf(os.Stderr, "Failed to write all data: %d != %d", n, len(data))
			}
		}()

		readData := make([]byte, len(data))
		n, err := io.ReadFull(serverSession, readData)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)

		require.Equal(t, data, readData)
	})

	t.Run("Control Read/Write", func(t *testing.T) {
		// Test sending a control message from the client to the server.
		expected := &v1alpha1.TerminalOpenRequest{
			Meta: message.Meta{
				APIVersion: v1alpha1.APIVersion,
				Kind:       "TerminalOpenRequest",
			},
			Columns: 80,
			Rows:    24,
			Env:     []string{"TERM=xterm-256color"},
		}
		err = clientSession.WriteControl(expected)
		require.NoError(t, err)

		actual := <-receivedOpenMessages
		require.Equal(t, expected, actual)
	})

	t.Run("Unimplemented Message Type", func(t *testing.T) {
		err := clientSession.WriteControl(new(v1alpha1.TerminalExit))
		require.ErrorIs(t, err, session.ErrUnimplementedMessageType)
	})
}
