// SPDX-License-Identifier: MIT
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 * Copyright (c) 2024 Yiwen AI
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

import { expect, describe, it } from "@jest/globals";
import { Xid } from "./index";

describe("xid", () => {
  it("new", () => {
    const xid = new Xid(new Uint8Array(12).fill(0));
    expect(xid.isZero()).toBeTruthy();
    expect(xid.toString()).toBe("00000000000000000000");
    expect(xid.timestamp()).toBe(0);
    expect(xid.pid()).toBe(0);
    expect(xid.counter()).toBe(0);
    expect(xid.toBytes().every((v) => v === 0)).toBeTruthy();
    expect(xid.machine().toString()).toBe("0,0,0");
    expect(xid.equals(Xid.parse("00000000000000000000"))).toBeTruthy();
    expect(xid.equals(Xid.default())).toBeTruthy();

    const now = Math.floor(Date.now() / 1000);
    const id1 = new Xid();
    const id2 = new Xid();
    expect(id1.isZero()).toBeFalsy();
    expect(id2.isZero()).toBeFalsy();
    expect(id1).not.toEqual(id2);
    expect(id1.timestamp()).toBeGreaterThanOrEqual(now);
    expect(id2.timestamp()).toBeGreaterThanOrEqual(now);
  });

  it("parse", () => {
    const cases = [
      ["64b78f6e73ee26338715e112", "cirourjjtoj371ols490"],
      ["64b78f6e73ee26338715e113", "cirourjjtoj371ols49g"],
      ["64b78f6e73ee26338715e114", "cirourjjtoj371ols4a0"],
      ["64b78f6e73ee26338715e115", "cirourjjtoj371ols4ag"],
      ["64b78f6e73ee26338715e116", "cirourjjtoj371ols4b0"],
      ["64b78f6e73ee26338715e117", "cirourjjtoj371ols4bg"],
    ];

    for (const v of cases) {
      const value = Uint8Array.from(
        v[0].match(/.{1,2}/g)?.map((byte) => parseInt(byte, 16)) || [],
      );
      const xid = Xid.fromValue(value);
      expect(xid.equals(Xid.parse(v[1]))).toBeTruthy();
      expect(xid.equals(new Xid(xid.toBytes()))).toBeTruthy();
    }
  });

  it("fromValue", () => {
    const xid = Xid.fromValue("9m4e2mr0ui3e8a215n4g");
    expect(xid.toString()).toBe("9m4e2mr0ui3e8a215n4g");

    expect(Xid.fromValue(xid)).toEqual(xid);
    expect(xid.equals(Xid.fromValue(xid))).toBeTruthy();

    expect(
      Xid.fromValue([
        0x4d, 0x88, 0xe1, 0x5b, 0x60, 0xf4, 0x86, 0xe4, 0x28, 0x41, 0x2d, 0xc9,
      ]),
    ).toEqual(xid);
    expect(
      Xid.fromValue(
        new Uint8Array([
          0x4d, 0x88, 0xe1, 0x5b, 0x60, 0xf4, 0x86, 0xe4, 0x28, 0x41, 0x2d,
          0xc9,
        ]),
      ),
    ).toEqual(xid);

    expect(() => Xid.fromValue("")).toThrow();
    expect(() => Xid.fromValue("00000000000000jarvis")).toThrow();
    expect(() => Xid.fromValue("0000000000000000000000000000")).toThrow();
    expect(() =>
      Xid.fromValue([
        0x4d, 0x88, 0xe1, 0x5b, 0x60, 0xf4, 0x86, 0xe4, 0x28, 0x41, 0x2d, 1999,
      ]),
    ).toThrow();
    expect(() =>
      Xid.fromValue(
        new Uint8Array([
          0x4d, 0x88, 0xe1, 0x5b, 0x60, 0xf4, 0x86, 0xe4, 0x28, 0x41, 0x2d,
        ]),
      ),
    ).toThrow();
  });

  it("json", () => {
    const xid = Xid.fromValue("9m4e2mr0ui3e8a215n4g");
    const obj = {
      id: xid,
      name: "yiwen",
    };
    const json = JSON.stringify(obj);
    expect(json).toBe('{"id":"9m4e2mr0ui3e8a215n4g","name":"yiwen"}');
    const obj1 = JSON.parse(json);
    expect(xid.equals(Xid.fromValue(obj1.id))).toBeTruthy();
  });
});
