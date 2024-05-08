// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package v1alpha1

import (
	"encoding/base64"

	"github.com/noisysockets/shell/internal/message"

	"github.com/rs/xid"
)

const (
	APIVersion = "nsh/v1alpha1"
)

// AckStatus is the status of an acknowledgment.
type AckStatus string

const (
	// AckStatusOK indicates the message was processed successfully.
	AckStatusOK AckStatus = "OK"
	// AckStatusError indicates an error occurred while processing the message.
	AckStatusError AckStatus = "ERROR"
	// AckStatusUnimplemented indicates the message was not processed because
	// the feature / message type is not implemented.
	AckStatusUnimplemented AckStatus = "UNIMPLEMENTED"
)

// Ack acknowledges a message has been received and processed.
type Ack struct {
	message.Meta `json:",inline"`
	// Status is the status of the acknowledgment.
	Status AckStatus `json:"status"`
	// Reason is the reason for the status, typically populated for errors.
	Reason string `json:"reason,omitempty"`
}

func (a *Ack) GetAPIVersion() string {
	return APIVersion
}

func (a *Ack) GetKind() string {
	return "Ack"
}

// Data is a message containing terminal data.
type Data struct {
	message.Meta `json:",inline"`
	// Data is the base64 encoded terminal data.
	Data string `json:"data"`
}

func (d *Data) GetAPIVersion() string {
	return APIVersion
}

func (d *Data) GetKind() string {
	return "Data"
}

// TerminalOpenRequest requests a new terminal session.
type TerminalOpenRequest struct {
	message.Meta `json:",inline"`
	// Columns is the number of columns in the terminal.
	Columns int `json:"columns"`
	// Rows is the number of rows in the terminal.
	Rows int `json:"rows"`
	// Env is a list of environment variables to pass to the shell process.
	// Uncontrolled setting of environment variables in a privileged process
	// can be a security hazard. Server implementations must validate the
	// environment variables before passing them to the shell process.
	// Typical safe values include TERM, LANG, and LC_*.
	Env []string `json:"env,omitempty"`
}

func (o *TerminalOpenRequest) GetAPIVersion() string {
	return APIVersion
}

func (o *TerminalOpenRequest) GetKind() string {
	return "TerminalOpenRequest"
}

// TerminalExit indicates the terminal session / shell has exited.
type TerminalExit struct {
	message.Meta `json:",inline"`
	// Status is the exit status of the shell process.
	Status int `json:"status"`
}

func (e *TerminalExit) GetAPIVersion() string {
	return APIVersion
}

func (e *TerminalExit) GetKind() string {
	return "TerminalExit"
}

// TerminalResize resizes the terminal window.
type TerminalResize struct {
	message.Meta `json:",inline"`
	// Columns is the number of columns in the terminal.
	Columns int `json:"columns"`
	// Rows is the number of rows in the terminal.
	Rows int `json:"rows"`
}

func (r *TerminalResize) GetAPIVersion() string {
	return APIVersion
}

func (r *TerminalResize) GetKind() string {
	return "TerminalResize"
}

// MaxDataMessageBytes is the maximum number of bytes that can be sent in a
// single data message.
var MaxDataMessageBytes int

func init() {
	message.RegisterType(func() message.Message { return &Ack{} })
	message.RegisterType(func() message.Message { return &Data{} })
	message.RegisterType(func() message.Message { return &TerminalOpenRequest{} })
	message.RegisterType(func() message.Message { return &TerminalExit{} })
	message.RegisterType(func() message.Message { return &TerminalResize{} })

	// Calculate the maximum data message size.
	b, err := message.Marshal(&Data{
		Meta: message.Meta{ID: xid.New().String()},
	})
	if err != nil {
		panic(err)
	}

	MaxDataMessageBytes = base64.StdEncoding.DecodedLen(message.MaxSize - len(b))
}
