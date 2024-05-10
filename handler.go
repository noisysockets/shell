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
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/noisysockets/shell/env"
	"github.com/noisysockets/shell/internal/message"
	"github.com/noisysockets/shell/internal/message/v1alpha1"
	"github.com/noisysockets/shell/internal/session"
	"github.com/noisysockets/shell/io"

	"github.com/creack/pty"
	"golang.org/x/sync/errgroup"

	"github.com/gorilla/websocket"
)

// Handler handles a shell session.
type Handler struct {
	logger      *slog.Logger
	session     *session.Session
	cmd         *exec.Cmd
	pty         *os.File
	tasks       *errgroup.Group
	tasksCtx    context.Context
	tasksCancel context.CancelFunc
}

// NewHandler creates a new shell handler and starts the session.
func NewHandler(ctx context.Context, logger *slog.Logger, ws *websocket.Conn) (*Handler, error) {
	logger = logger.WithGroup("handler")

	ctx, tasksCancel := context.WithCancel(ctx)

	tasks, tasksCtx := errgroup.WithContext(ctx)

	h := &Handler{
		logger:      logger,
		tasks:       tasks,
		tasksCtx:    tasksCtx,
		tasksCancel: tasksCancel,
	}

	messageHandlers := map[string]session.MessageHandler{
		message.Type(new(v1alpha1.TerminalOpenRequest)): h.handleTerminalOpenRequest,
		message.Type(new(v1alpha1.TerminalResize)):      h.handleTerminalResize,
	}

	var err error
	h.session, err = session.NewSession(ctx, logger, ws, &session.Config{
		MessageHandlers: messageHandlers,
	})

	h.tasks.Go(func() error {
		go func() {
			<-tasksCtx.Done()

			logger.Debug("Closing session")

			_ = h.session.Close()
		}()

		err := h.session.Wait()
		logger.Debug("Session completed", slog.Any("error", err))
		if err != nil {
			return err
		}

		// Always return at least a cancelled error (so that the errgroup.Group
		// will complete).
		return context.Canceled
	})

	return h, err
}

// Close closes the handler.
func (h *Handler) Close() error {
	h.logger.Debug("Cancelling background tasks")

	h.tasksCancel()

	// The PTY will be closed by the background cleanup tasks.

	if err := h.tasks.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

// Wait waits for the handler to complete (including all background tasks).
func (h *Handler) Wait() error {
	if err := h.tasks.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

func (h *Handler) handleTerminalOpenRequest(msg message.Message) (bool, error) {
	req := msg.(*v1alpha1.TerminalOpenRequest)

	if h.pty != nil {
		return true, fmt.Errorf("PTY already opened")
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return true, fmt.Errorf("failed to get user home directory: %w", err)
	}

	h.cmd = exec.CommandContext(h.tasksCtx, shell)
	h.cmd.Dir = homeDir
	h.cmd.Env = append(env.Default(), env.FilterSafe(req.Env)...)

	h.pty, err = pty.StartWithSize(h.cmd, &pty.Winsize{
		Cols: uint16(req.Columns),
		Rows: uint16(req.Rows),
	})
	if err != nil {
		return true, fmt.Errorf("failed to open PTY: %w", err)
	}

	if err := syscall.SetNonblock(int(h.pty.Fd()), true); err != nil {
		return true, fmt.Errorf("failed to set PTY to non-blocking mode: %w", err)
	}

	h.startCopying()

	return true, nil
}

func (h *Handler) handleTerminalResize(msg message.Message) (bool, error) {
	req := msg.(*v1alpha1.TerminalResize)

	if h.pty == nil {
		return true, fmt.Errorf("PTY not allocated")
	}

	if err := pty.Setsize(h.pty, &pty.Winsize{
		Cols: uint16(req.Columns),
		Rows: uint16(req.Rows),
	}); err != nil {
		return true, fmt.Errorf("failed to resize PTY: %w", err)
	}

	return true, nil
}

func (h *Handler) startCopying() {
	// If either direction is closed, we'll abort both.
	ctx, cancel := context.WithCancel(h.tasksCtx)

	var doneCopyingMu sync.Mutex
	var doneCopying bool

	h.tasks.Go(func() error {
		<-ctx.Done()

		h.logger.Debug("Finished copying to and from PTY")

		doneCopyingMu.Lock()
		doneCopying = true
		if err := h.pty.Close(); err != nil {
			doneCopyingMu.Unlock()
			return fmt.Errorf("failed to close PTY: %w", err)
		}
		h.pty = nil
		doneCopyingMu.Unlock()

		h.logger.Debug("Waiting for shell process to exit")

		state, err := h.cmd.Process.Wait()
		if err != nil {
			return fmt.Errorf("failed to wait for shell process: %w", err)
		}

		status, ok := state.Sys().(syscall.WaitStatus)
		if !ok {
			h.logger.Debug("Failed to get exit status from shell process")

			return h.session.WriteControl(&v1alpha1.TerminalExit{})
		}

		exitStatus := status.ExitStatus()
		if status.Signaled() {
			exitStatus = 0
		}

		h.logger.Debug("Process exited", slog.Any("exit_status", exitStatus))

		return h.session.WriteControl(&v1alpha1.TerminalExit{
			Status: exitStatus,
		})
	})

	h.tasks.Go(func() error {
		defer cancel()
		_, err := io.CopyContext(ctx, h.pty, h.session)
		h.logger.Debug("Finished copying data to PTY", slog.Any("error", err))

		doneCopyingMu.Lock()
		defer doneCopyingMu.Unlock()

		if !doneCopying && err != nil {
			return fmt.Errorf("failed to copy data to PTY: %w", err)
		}

		return nil
	})

	h.tasks.Go(func() error {
		defer cancel()
		_, err := io.CopyContext(ctx, h.session, h.pty)
		h.logger.Debug("Finished copying data from PTY", slog.Any("error", err))

		doneCopyingMu.Lock()
		defer doneCopyingMu.Unlock()

		if !doneCopying && err != nil {
			return fmt.Errorf("failed to copy data from PTY: %w", err)
		}

		return nil
	})
}
