// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

// Package io provides context-aware I/O operations.
package io

import (
	stdio "io"
	"time"
)

// Reader types.
type DeadlineReader interface {
	stdio.Reader
	SetReadDeadline(t time.Time) error
}

type DeadlineReadCloser interface {
	DeadlineReader
	stdio.Closer
}

// Writer types.
type DeadlineWriter interface {
	stdio.Writer
	SetWriteDeadline(t time.Time) error
}

type DeadlineWriteCloser interface {
	DeadlineWriter
	stdio.Closer
}
