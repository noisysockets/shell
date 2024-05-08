// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package shell

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/noisysockets/shell/internal/io"
	"github.com/noisysockets/shell/internal/message"
	"github.com/noisysockets/shell/internal/message/v1alpha1"
	"github.com/noisysockets/shell/internal/session"
	"golang.org/x/sync/errgroup"

	"github.com/gorilla/websocket"
)

// ExitHandler is a callback function that is called when the remote shell
// process exits.
type ExitHandler func(exitStatus int) error

// Client is a remote shell client.
type Client struct {
	logger      *slog.Logger
	session     *session.Session
	tasks       *errgroup.Group
	tasksCtx    context.Context
	tasksCancel context.CancelFunc
	onExit      ExitHandler
}

// NewClient creates a new remote shell client and starts the session.
func NewClient(ctx context.Context, logger *slog.Logger, ws *websocket.Conn) (*Client, error) {
	ctx, tasksCancel := context.WithCancel(ctx)

	tasks, tasksCtx := errgroup.WithContext(ctx)

	c := &Client{
		logger:      logger.WithGroup("client"),
		tasks:       tasks,
		tasksCtx:    tasksCtx,
		tasksCancel: tasksCancel,
	}

	messageHandlers := map[string]session.MessageHandler{
		message.Type(new(v1alpha1.TerminalExit)): c.handleTerminalExit,
	}

	var err error
	c.session, err = session.NewSession(ctx, logger, ws, &session.Config{
		MessageHandlers: messageHandlers,
	})
	if err != nil {
		return nil, err
	}

	c.tasks.Go(func() error {
		go func() {
			<-tasksCtx.Done()

			logger.Debug("Closing session")

			_ = c.session.Close()
		}()

		err := c.session.Wait()
		logger.Debug("Session completed", slog.Any("error", err))
		if err != nil {
			return err
		}

		// Always return at least a cancelled error (so that the errgroup.Group
		// will complete).
		return context.Canceled
	})

	return c, nil
}

// Close closes the client.
func (c *Client) Close() error {
	c.logger.Debug("Cancelling background tasks")

	// Cancel the background tasks.
	c.tasksCancel()

	// Wait for the background tasks to complete.
	if err := c.tasks.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

// Wait waits for the client to complete (including all background tasks).
func (c *Client) Wait() error {
	if err := c.tasks.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

// OpenTerminal opens a new terminal.
func (c *Client) OpenTerminal(columns, rows int, env []string,
	input io.Reader, output io.Writer, onExit ExitHandler) error {
	err := c.session.WriteControl(&v1alpha1.TerminalOpenRequest{
		Columns: columns,
		Rows:    rows,
		Env:     env,
	})
	if err != nil {
		return err
	}

	// If either direction is closed, or the terminal exits we'll abort both.
	ctx, cancel := context.WithCancel(c.tasksCtx)

	c.onExit = func(exitStatus int) error {
		cancel()

		return onExit(exitStatus)
	}

	// Start copying data to/from the remote shell.
	c.tasks.Go(func() error {
		defer cancel()
		_, err := io.CopyContext(ctx, c.session, input)
		c.logger.Debug("Finished copying data to remote shell", slog.Any("error", err))
		if err != nil {
			return fmt.Errorf("failed to copy data to remote shell: %w", err)
		}

		return nil
	})

	c.tasks.Go(func() error {
		defer cancel()
		_, err := io.CopyContext(ctx, output, c.session)
		c.logger.Debug("Finished copying data from remote shell", slog.Any("error", err))
		if err != nil {
			return fmt.Errorf("failed to copy data from remote shell: %w", err)
		}

		return nil
	})

	return nil
}

// ResizeTerminal resizes the terminal window.
func (c *Client) ResizeTerminal(columns, rows int) error {
	return c.session.WriteControl(&v1alpha1.TerminalResize{
		Columns: columns,
		Rows:    rows,
	})
}

func (c *Client) handleTerminalExit(msg message.Message) (bool, error) {
	ptyExited := msg.(*v1alpha1.TerminalExit)

	if c.onExit != nil {
		defer func() {
			c.onExit = nil
		}()

		if err := c.onExit(ptyExited.Status); err != nil {
			return true, err
		}
	}

	return true, nil
}
