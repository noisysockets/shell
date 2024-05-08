/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

export const APIVersion: string = "nsh/v1alpha1";

interface Meta {
  // apiVersion is the version of the API.
  apiVersion: string;
  // kind is the kind of message.
  kind: string;
  // id is the globally unique message ID, or in the case of an Ack,
  // the ID of the message being acknowledged.
  id?: string;
}

// AckStatus is the status of an acknowledgment.
export const enum AckStatus {
  // OK indicates the message was processed successfully.
  OK = "OK",
  // ERROR indicates an error occurred while processing the message.
  ERROR = "ERROR",
  // UNIMPLEMENTED indicates the message was not processed because the
  // feature / message type is not implemented.
  UNIMPLEMENTED = "UNIMPLEMENTED",
}

// Ack acknowledges a message has been received and processed.
interface Ack extends Meta {
  // status is the status of the acknowledgment.
  status: AckStatus;
  // reason is the reason for the status, typically populated for errors.
  reason: string;
}

// Data is a message containing terminal data.
interface Data extends Meta {
  // data is the base64 encoded terminal data.
  data: string;
}

// TerminalOpenRequest requests a new terminal session.
interface TerminalOpenRequest extends Meta {
  // columns is the number of columns in the terminal.
  columns: number;
  // rows is the number of rows in the terminal.
  rows: number;
  // env is a list of environment variables to pass to the shell process.
  env: string[];
}

// TerminalExit indicates the terminal session / shell has exited.
interface TerminalExit extends Meta {
  // status is the exit status of the shell process.
  status: number;
}

// TerminalResize resizes the terminal window.
interface TerminalResize extends Meta {
  // columns is the number of columns in the terminal.
  columns: number;
  // rows is the number of rows in the terminal.
  rows: number;
}

// MaxDataMessageBytes is the maximum number of bytes that can be sent in a
// single data message. This has been cribbed from the Go implementation.
export const MaxDataMessageBytes: number = 49089;

export type Message =
  | Ack
  | Data
  | TerminalOpenRequest
  | TerminalExit
  | TerminalResize;

export type {
  Meta,
  Ack,
  Data,
  TerminalOpenRequest,
  TerminalExit,
  TerminalResize,
};
