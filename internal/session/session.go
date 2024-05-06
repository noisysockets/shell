// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package session

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/noisysockets/shell/internal/io"
	"github.com/noisysockets/shell/internal/message"
	"github.com/noisysockets/shell/internal/message/v1alpha1"

	"github.com/gorilla/websocket"
	"github.com/rs/xid"
	"golang.org/x/sync/errgroup"
)

const (
	heartbeatInterval = time.Minute
	heartbeatTimeout  = 30 * time.Second
	netTimeout        = 10 * time.Second
	ackTimeout        = 30 * time.Second
)

var (
	ErrAckTimeout               = fmt.Errorf("timed out waiting for ack")
	ErrHeartbeatTimeout         = fmt.Errorf("timed out waiting for heartbeat reply")
	ErrUnimplementedMessageType = fmt.Errorf("unimplemented message type")
)

// MessageHandler is a function that handles messages.
type MessageHandler func(message.Message) (sendAck bool, err error)

// Session represents an noisy shell session.
type Session struct {
	logger                 *slog.Logger
	ws                     *websocket.Conn
	dataReader             io.ReadCloser
	dataWriter             io.WriteCloser
	tasks                  *errgroup.Group
	tasksCtx               context.Context
	tasksCancel            context.CancelFunc
	messageHandlersMu      sync.Mutex
	messageHandlersForType map[string]MessageHandler
	messageHandlersForID   map[string]MessageHandler
	writeDeadline          *time.Time
	sendMessageMu          sync.Mutex
}

// NewSession creates a new noisy shell session.
func NewSession(ctx context.Context, logger *slog.Logger, ws *websocket.Conn,
	messageHandlers map[string]MessageHandler) (*Session, error) {
	logger = logger.WithGroup("session")

	ws.SetReadLimit(message.MaxSize)

	ws.SetCloseHandler(func(code int, text string) error {
		logger.Debug("Received close message",
			slog.Int("code", code),
			slog.String("text", text))

		return &websocket.CloseError{
			Code: code,
			Text: text,
		}
	})

	dataReader, dataWriter := io.Pipe()

	ctx, tasksCancel := context.WithCancel(ctx)

	tasks, tasksCtx := errgroup.WithContext(ctx)

	s := &Session{
		logger:                 logger,
		ws:                     ws,
		dataReader:             dataReader,
		dataWriter:             dataWriter,
		tasks:                  tasks,
		tasksCtx:               tasksCtx,
		tasksCancel:            tasksCancel,
		messageHandlersForType: make(map[string]MessageHandler),
		messageHandlersForID:   make(map[string]MessageHandler),
	}

	s.messageHandlersForType[message.Type(new(v1alpha1.Ack))] = func(msg message.Message) (bool, error) {
		ackMsg := msg.(*v1alpha1.Ack)

		logger.Debug("Unhandled ack",
			slog.String("id", ackMsg.GetID()),
			slog.String("status", string(ackMsg.Status)),
			slog.String("reason", ackMsg.Reason))

		return false, nil
	}

	// Register the data message handler.
	s.messageHandlersForType[message.Type(new(v1alpha1.Data))] = func(msg message.Message) (bool, error) {
		dataMsg := msg.(*v1alpha1.Data)

		chunk, err := base64.StdEncoding.DecodeString(dataMsg.Data)
		if err != nil {
			return false, fmt.Errorf("failed to decode data: %w", err)
		}

		logger.Debug("Received data",
			slog.String("id", dataMsg.GetID()),
			slog.Int("size", len(chunk)))

		if _, err := io.CopyContext(tasksCtx, s.dataWriter, io.NopDeadlineReader(bytes.NewReader(chunk))); err != nil {
			return false, fmt.Errorf("failed to write data: %w", err)
		}

		return false, nil
	}

	// Register user supplied message handlers (if present).
	for tm, handler := range messageHandlers {
		s.messageHandlersForType[tm] = handler
	}

	s.tasks.Go(s.processMessages)
	s.tasks.Go(s.sendHeartbeats)

	return s, nil
}

// Close closes the session.
func (s *Session) Close() error {
	s.logger.Debug("Cancelling background tasks")

	s.tasksCancel()

	tasksErr := ignoreExpectedError(s.tasks.Wait())

	s.logger.Debug("Attempting to send close message to peer", slog.Any("error", tasksErr))

	closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	if tasksErr != nil {
		closeMessage = websocket.FormatCloseMessage(websocket.CloseInternalServerErr, tasksErr.Error())
	}

	if err := s.ws.WriteControl(websocket.CloseMessage, closeMessage, time.Now().Add(netTimeout)); err != nil {
		s.logger.Debug("Failed to send close message to peer", slog.Any("error", err))
	}

	s.logger.Debug("Closing data pipe")

	if err := s.dataReader.Close(); err != nil {
		return fmt.Errorf("failed to close data pipe: %w", err)
	}

	if err := s.dataWriter.Close(); err != nil {
		return fmt.Errorf("failed to close data pipe: %w", err)
	}

	s.logger.Debug("Closing websocket connection")

	if err := s.ws.Close(); err != nil {
		return fmt.Errorf("failed to close websocket connection: %w", err)
	}

	return tasksErr
}

// Wait waits for the session to complete (including all background tasks).
func (s *Session) Wait() error {
	return ignoreExpectedError(s.tasks.Wait())
}

// Read reads data from the session
func (s *Session) Read(b []byte) (n int, err error) {
	return s.dataReader.Read(b)
}

// Write writes data to the session.
func (s *Session) Write(b []byte) (n int, err error) {
	defer func() { s.writeDeadline = nil }()

	chunk := make([]byte, v1alpha1.MaxDataMessageBytes)
	for len(b) > 0 {
		if s.writeDeadline != nil && time.Now().After(*s.writeDeadline) {
			return n, os.ErrDeadlineExceeded
		}

		chunkSize := min(len(b), v1alpha1.MaxDataMessageBytes)
		copy(chunk, b[:chunkSize])
		b = b[chunkSize:]

		if err := s.sendMessage(&v1alpha1.Data{
			Meta: message.Meta{ID: xid.New().String()},
			Data: base64.StdEncoding.EncodeToString(chunk[:chunkSize]),
		}); err != nil {
			return n, fmt.Errorf("failed to send chunk: %w", err)
		}

		n += chunkSize
	}

	return n, nil
}

// WriteControl sends a control message and waits for an acknowledgment.
func (s *Session) WriteControl(msg message.Message) error {
	result := make(chan error, 1)
	defer close(result)

	id := xid.New().String()
	msg.SetID(id)

	handler := func(msg message.Message) (bool, error) {
		defer func() {
			// Don't panic if the channel is closed.
			if r := recover(); r != nil {
				s.logger.Debug("Caught panic in message handler",
					slog.Any("recovered", r))
			}
		}()

		ackMsg, ok := msg.(*v1alpha1.Ack)
		if !ok {
			return false, fmt.Errorf("expected an ack, got: %s", message.Type(msg))
		}

		switch ackMsg.Status {
		case v1alpha1.AckStatusOK:
			result <- nil
		case v1alpha1.AckStatusError:
			result <- fmt.Errorf("error with reason: %s", ackMsg.Reason)
		case v1alpha1.AckStatusUnimplemented:
			result <- ErrUnimplementedMessageType
		default:
			result <- fmt.Errorf("unexpected status value in ack: %s", ackMsg.Status)
		}

		return false, nil
	}

	s.messageHandlersMu.Lock()
	s.messageHandlersForID[id] = handler
	s.messageHandlersMu.Unlock()

	if err := s.sendMessage(msg); err != nil {
		return err
	}

	select {
	case err := <-result:
		return err
	case <-time.After(ackTimeout):
		s.messageHandlersMu.Lock()
		delete(s.messageHandlersForID, id)
		s.messageHandlersMu.Unlock()

		return ErrAckTimeout
	case <-s.tasksCtx.Done():
		return s.tasksCtx.Err()
	}
}

// SetReadDeadline sets the read deadline on the session.
func (s *Session) SetReadDeadline(t time.Time) error {
	return s.dataReader.(io.DeadlineReader).SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline on the session.
func (s *Session) SetWriteDeadline(t time.Time) error {
	s.writeDeadline = &t
	return nil
}

func (s *Session) sendMessage(msg message.Message) error {
	msgData, err := message.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	s.sendMessageMu.Lock()
	defer s.sendMessageMu.Unlock()

	s.logger.Debug("Sending message",
		slog.String("id", msg.GetID()),
		slog.String("type", message.Type(msg)),
		slog.Int("size", len(msgData)))

	if err := s.ws.SetWriteDeadline(time.Now().Add(netTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	if err := s.ws.WriteMessage(websocket.TextMessage, msgData); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

func (s *Session) processMessages() error {
	type closeReader interface {
		CloseRead() error
	}

	s.logger.Debug("Processing messages")
	defer s.logger.Debug("Stopped processing messages")

	s.tasks.Go(func() error {
		<-s.tasksCtx.Done()

		// Break out of the ReadMessage loop.
		if cr, ok := s.ws.NetConn().(closeReader); ok {
			s.logger.Debug("Closing read side of websocket connection")

			if err := cr.CloseRead(); err != nil && !(errors.Is(err, net.ErrClosed) || errors.Is(err, syscall.ENOTCONN)) {
				s.logger.Debug("Failed to close read side of websocket connection", slog.Any("error", err))
				return err
			}
		} else {
			// Peer won't receive close messages sadly.
			s.logger.Debug("Closing websocket connection")

			if err := s.ws.Close(); err != nil {
				s.logger.Debug("Failed to close websocket connection", slog.Any("error", err))

				return err
			}
		}

		return nil
	})

	for {
		_, msgData, err := s.ws.ReadMessage()
		if err != nil {
			s.logger.Debug("Failed to read message", slog.Any("error", err))
			// Has the task context been cancelled?
			select {
			case <-s.tasksCtx.Done():
				if strings.Contains(err.Error(), "unexpected EOF") {
					return &websocket.CloseError{Code: websocket.CloseNormalClosure}
				}
				return err
			default:
			}

			s.logger.Debug("Failed to read message", slog.Any("error", err))

			return err
		}

		msg, err := message.Unmarshal(msgData)
		if err != nil {
			s.logger.Warn("Failed to unmarshal message", slog.Any("error", err))

			continue
		}

		typ := message.Type(msg)

		logger := s.logger.With(
			slog.String("id", msg.GetID()),
			slog.String("type", typ))

		logger.Debug("Received message", slog.Int("size", len(msgData)))

		s.messageHandlersMu.Lock()
		handler, ok := s.messageHandlersForID[msg.GetID()]
		if ok {
			delete(s.messageHandlersForID, msg.GetID())
		}

		if !ok {
			handler, ok = s.messageHandlersForType[typ]
		}
		s.messageHandlersMu.Unlock()

		if ok {
			logger.Debug("Found handler for message")

			if sendAck, err := handler(msg); err != nil {
				logger.Warn("Error handling message", slog.Any("error", err))

				if err := s.sendMessage(&v1alpha1.Ack{
					Meta:   message.Meta{ID: msg.GetID()},
					Status: v1alpha1.AckStatusError,
					Reason: err.Error(),
				}); err != nil {
					return fmt.Errorf("failed to send ack: %w", err)
				}
			} else if sendAck {
				logger.Debug("Finished handling message, sending ack")

				if err := s.sendMessage(&v1alpha1.Ack{
					Meta:   message.Meta{ID: msg.GetID()},
					Status: v1alpha1.AckStatusOK,
				}); err != nil {
					return fmt.Errorf("failed to send ack: %w", err)
				}
			} else {
				logger.Debug("Finished handling message")
			}
		} else {
			s.logger.Warn("Unimplemented message type", slog.String("type", typ))

			if err := s.sendMessage(&v1alpha1.Ack{
				Meta:   message.Meta{ID: msg.GetID()},
				Status: v1alpha1.AckStatusUnimplemented,
			}); err != nil {
				return fmt.Errorf("failed to send ack: %w", err)
			}
		}
	}
}

func (s *Session) sendHeartbeats() error {
	s.logger.Debug("Sending regular heartbeats")
	defer s.logger.Debug("Stopped sending regular heartbeats")

	ticker := time.NewTicker(heartbeatInterval)

	for {
		select {
		case <-ticker.C:
			id := xid.New().String()

			s.logger.Debug("Sending heartbeat", slog.String("id", id))

			received := make(chan struct{})
			s.ws.SetPongHandler(func(receivedID string) error {
				if receivedID != id {
					s.logger.Warn("Unexpected heartbeat id",
						slog.String("expected", id),
						slog.String("received", receivedID))

					return fmt.Errorf("unexpected heartbeat id: %s", id)
				}

				s.logger.Debug("Received heartbeat reply", slog.String("id", id))

				close(received)

				return nil
			})

			if err := s.ws.WriteControl(websocket.PingMessage, []byte(id),
				time.Now().Add(netTimeout)); err != nil {
				return fmt.Errorf("failed to send heartbeat: %w", err)
			}

			select {
			case <-received:
			case <-time.After(heartbeatTimeout):
				return ErrHeartbeatTimeout
			case <-s.tasksCtx.Done():
				return s.tasksCtx.Err()
			}
		case <-s.tasksCtx.Done():
			return s.tasksCtx.Err()
		}
	}
}

func ignoreExpectedError(err error) error {
	if errors.Is(err, context.Canceled) ||
		websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return nil
	}

	return err
}
