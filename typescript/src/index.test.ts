/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

import { expect, describe, it } from "@jest/globals";
import { EventEmitter } from "eventemitter3";

import { Client } from "./index";

describe("Noisy Sockets Shell", () => {
  it("hello", async () => {
    const input = new EventEmitter();
    const output = new EventEmitter();

    const client = new Client("ws://localhost:8080/ws");

    let onExit: (exitStatus: number) => void;
    const exitPromise = new Promise<number>((resolve) => {
      onExit = resolve;
    });

    const waitForPrompt = readWithTimeout(output, 1000);

    // Open a terminal.
    await client.openTerminal(
      80,
      24,
      ["TERM=dumb"],
      input,
      output,
      (exitStatus: number) => {
        onExit(exitStatus);
      },
    );

    // Wait for the prompt.
    await waitForPrompt;

    const dateOutput = readWithTimeout(output, 1000, 2);

    // Send a `date` command.
    const dateCommand = "date --rfc-3339=s\n";
    input.emit("data", new TextEncoder().encode(dateCommand));

    const currentDate = (await dateOutput).split("\n")[1];

    // Is the date valid?
    expect(Date.parse(currentDate)).toBeGreaterThan(0);

    // Close the terminal.
    const exitCommand = "exit 2\n";
    input.emit("data", new TextEncoder().encode(exitCommand));

    // Wait for the terminal to close.
    const exitStatus = await exitPromise;

    expect(exitStatus).toBe(2);

    // Close the client.
    client.close();
  });
});

const readWithTimeout = async (
  output: EventEmitter,
  timeoutMS: number,
  maxLines?: number,
): Promise<string> => {
  let outputStr = "";
  return new Promise<string>((resolve) => {
    const listener = (data: Uint8Array) => {
      outputStr += new TextDecoder().decode(data);

      if (maxLines && outputStr.split("\n").length > maxLines) {
        output.off("data", listener);
        resolve(outputStr);
      }
    };
    output.on("data", listener);

    setTimeout(() => {
      output.off("data", listener);
      resolve(outputStr);
    }, timeoutMS);
  });
};
