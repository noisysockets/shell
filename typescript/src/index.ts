/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

import EventEmitter from "eventemitter3";
import { Xid } from "./xid/index";

import {
  APIVersion,
  AckStatus,
  Ack,
  Data,
  TerminalExit,
  Message,
  TerminalOpenRequest,
  TerminalResize,
  MaxDataMessageBytes,
} from "./message";

class Client {
  private ws: WebSocket;
  private input?: EventEmitter;
  private output?: EventEmitter;
  private handleInputData?: (data: Uint8Array) => void;
  private messageHandlersForID: Map<string, (msg: Message) => void> = new Map();
  private onExit?: (exitStatus: number) => void;
  private connected: Promise<void>;

  constructor(url: string) {
    this.ws = new WebSocket(url);

    this.ws.onclose = () => {
      this.close();
    };

    this.ws.onmessage = (event: MessageEvent) => {
      this.handleMessage(event);
    };

    let resolveConnected: () => void;
    let rejectConnected: (e: Error) => void;
    this.connected = new Promise<void>((resolve, reject) => {
      resolveConnected = resolve;
      rejectConnected = reject;
    });

    this.ws.onopen = () => {
      resolveConnected();
    };

    this.ws.onerror = (event: Event) => {
      rejectConnected(new Error(`WebSocket error: ${event}`));
    };
  }

  public close() {
    if (this.input !== undefined && this.handleInputData !== undefined) {
      this.input.off("data", this.handleInputData);
    }
    this.input = undefined;
    this.output = undefined;

    this.ws.close();
  }

  // openTerminal opens a new terminal.
  public async openTerminal(
    columns: number,
    rows: number,
    env: string[],
    input: EventEmitter,
    output: EventEmitter,
    onExit?: (exitStatus: number) => void,
  ): Promise<void> {
    const msg: TerminalOpenRequest = {
      apiVersion: APIVersion,
      kind: "TerminalOpenRequest",
      columns: columns,
      rows: rows,
      env: env,
    };

    await this.send(msg, true);

    // Set up exit handler.
    this.onExit = onExit;

    // Start copying terminal data.
    this.handleInputData = (data: Uint8Array) => {
      for (let i = 0; i < data.length; i += MaxDataMessageBytes) {
        const chunk = data.slice(i, i + MaxDataMessageBytes);

        const msg: Data = {
          apiVersion: APIVersion,
          kind: "Data",
          data: btoa(String.fromCharCode(...chunk)),
        };
        this.send(msg, false);
      }
    };
    input.on("data", this.handleInputData);

    this.output = output;

    return Promise.resolve();
  }

  // resizeTerminal resizes the terminal window.
  public resizeTerminal(columns: number, rows: number): Promise<void> {
    const msg: TerminalResize = {
      apiVersion: APIVersion,
      kind: "TerminalResize",
      columns: columns,
      rows: rows,
    };
    return this.send(msg, true);
  }

  private handleMessage(event: MessageEvent) {
    const msg: Message = JSON.parse(event.data);

    if (msg.id !== undefined) {
      const handler = this.messageHandlersForID.get(msg.id);
      if (handler !== undefined) {
        handler(msg);
        return;
      }
    }

    const msgType = `${msg.apiVersion}/${msg.kind}`;
    switch (msgType) {
      case "nsh/v1alpha1/Data":
        this.handleData(msg as Data);
        break;
      case "nsh/v1alpha1/TerminalExit":
        this.handleTerminalExit(msg as TerminalExit);
        break;
      case "nsh/v1alpha1/Ack":
        // Ignore ack messages that are not in response to a request.
        console.warn(`Unhandled Ack message: ${msg.id}`)
        break;
      default:
        console.error(`Unexpected message: ${msgType}`);
        this.sendAck(msg, AckStatus.UNIMPLEMENTED);
    }
  }

  private handleData(msg: Data) {
    const data = Uint8Array.from(atob(msg.data), (c) => c.charCodeAt(0));
    if (this.output !== undefined) {
      this.output.emit("data", data);
    }
  }

  private handleTerminalExit(msg: TerminalExit) {
    // Stop copying terminal data.
    if (this.input !== undefined && this.handleInputData !== undefined) {
      this.input.off("data", this.handleInputData);
    }

    this.input = undefined;
    this.output = undefined;

    if (this.onExit !== undefined) {
      this.onExit(msg.status);
      this.onExit = undefined;
    }

    this.sendAck(msg, AckStatus.OK);
  }

  private async send(msg: Message, wantAck: boolean): Promise<void> {
    await this.connected;

    msg.id = new Xid().toString();
    this.ws.send(JSON.stringify(msg));

    if (!wantAck) {
      return Promise.resolve();
    }

    return new Promise<void>((resolve, reject) => {
      this.messageHandlersForID.set(msg.id!, (msg: Message) => {
        this.messageHandlersForID.delete(msg.id!);

        if (msg.kind === "Ack") {
          const ack = msg as Ack;
          if (ack.status === AckStatus.OK) {
            resolve();
          } else {
            reject(`Error: ${ack.reason}`);
          }
        } else {
          reject(`Unexpected message: ${msg.apiVersion}.${msg.kind}`);
        }
      });
    });
  }

  private async sendAck(msg: Message, status: AckStatus, reason?: string): Promise<void> {
    await this.connected;

    const ack: Ack = {
      apiVersion: APIVersion,
      kind: "Ack",
      id: msg.id,
      status: status,
      reason: reason,
    };

    this.ws.send(JSON.stringify(ack));

    return Promise.resolve();
  }
}

export { Client };
