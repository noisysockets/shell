/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package shell_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	stdio "io"
	"os"
	"os/user"
	"strings"
	"time"

	"testing"

	"github.com/gorilla/websocket"
	"github.com/neilotoole/slogt"
	"github.com/noisysockets/shell"
	"github.com/noisysockets/shell/internal/testutil"
	"github.com/noisysockets/shell/io"
	"github.com/stretchr/testify/require"
)

func TestShell(t *testing.T) {
	logger := slogt.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	addr, serverWsChan, err := testutil.StartServer(ctx)
	require.NoError(t, err)

	// Connect to the server.
	clientWs, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/shell/ws", addr), nil)
	require.NoError(t, err)

	// Create a new client.
	client, err := shell.NewClient(ctx, logger, clientWs)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	// Get the server's websocket connection.
	serverWs := <-serverWsChan

	// Create a new handler.
	handler, err := shell.NewHandler(ctx, logger, serverWs)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, handler.Close())
	})

	exitStatusChan := make(chan int, 1)
	t.Cleanup(func() {
		close(exitStatusChan)
	})

	onExit := func(exitStatus int) error {
		exitStatusChan <- exitStatus
		return nil
	}

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()

	// Use an os.Pipe to buffer the output.
	bufferedOutputReader, bufferedOutputWriter, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bufferedOutputReader.Close())
		require.NoError(t, bufferedOutputWriter.Close())
	})

	// Pump the output from the shell to the buffered output.
	// As the shell output isn't buffered.
	go func() {
		_, _ = io.CopyContext(ctx, bufferedOutputWriter, outputReader)
	}()

	// Open a new dumb terminal.
	err = client.OpenTerminal(80, 24, []string{"TERM=dumb"},
		inputReader, outputWriter, onExit)
	require.NoError(t, err)

	// Resize the terminal.
	err = client.ResizeTerminal(132, 24)
	require.NoError(t, err)

	// Clear the output (eg. the initial prompt).
	_, err = readWithTimeout(ctx, bufferedOutputReader, time.Second)
	require.NoError(t, err)

	// Send a command to the shell (get the current user).
	_, err = stdio.WriteString(inputWriter, "whoami\n")
	require.NoError(t, err)

	// Read the output of the command.
	commandOutput, err := readWithTimeout(ctx, bufferedOutputReader, time.Second)
	require.NoError(t, err)

	lines := strings.Split(commandOutput, "\n")
	require.Len(t, lines, 3)

	// Ensure the output of the command is correct.
	username := strings.TrimSpace(string(lines[1]))

	cur, err := user.Current()
	require.NoError(t, err)

	require.Equal(t, cur.Username, username)

	// Send an exit command.
	_, err = stdio.WriteString(inputWriter, "exit 2\n")
	require.NoError(t, err)

	// Wait for the exit status.
	select {
	case exitStatus := <-exitStatusChan:
		require.Equal(t, 2, exitStatus)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shell to exit")
	}
}

func readWithTimeout(ctx context.Context, r io.DeadlineReader, timeout time.Duration) (string, error) {
	readCtx, readCancel := context.WithTimeout(ctx, timeout)
	defer readCancel()

	var buf bytes.Buffer
	_, err := io.CopyContext(readCtx, io.NopDeadlineWriter(&buf), r)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return "", err
	}

	return buf.String(), nil
}
