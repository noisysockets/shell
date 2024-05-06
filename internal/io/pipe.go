// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 * Portions of this file are based on code originally from the Go project,
 *
 * Copyright (c) 2009 The Go Authors. All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 *   * Redistributions of source code must retain the above copyright
 *     notice, this list of conditions and the following disclaimer.
 *   * Redistributions in binary form must reproduce the above
 *     copyright notice, this list of conditions and the following disclaimer
 *     in the documentation and/or other materials provided with the
 *     distribution.
 *   * Neither the name of Google Inc. nor the names of its
 *     contributors may be used to endorse or promote products derived from
 *     this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package io

import (
	stdio "io"
	"os"
	"sync"
	"time"
)

var ErrClosedPipe = stdio.ErrClosedPipe

type pipe struct {
	wrMu            sync.Mutex
	wrCh            chan []byte
	rdCh            chan int
	doneOnce        sync.Once
	done            chan struct{}
	readDeadlineMu  sync.Mutex
	readDeadline    *time.Time
	writeDeadlineMu sync.Mutex
	writeDeadline   *time.Time
}

func (p *pipe) read(b []byte) (n int, err error) {
	var C <-chan time.Time

	p.readDeadlineMu.Lock()
	readDeadline := p.readDeadline
	p.readDeadlineMu.Unlock()

	if readDeadline != nil {
		defer func() {
			p.readDeadlineMu.Lock()
			p.readDeadline = nil
			p.readDeadlineMu.Unlock()
		}()
		d := time.Until(*readDeadline)
		if d <= 0 {
			return 0, os.ErrDeadlineExceeded
		}
		timer := time.NewTimer(d)
		C = timer.C
		defer timer.Stop()
	} else {
		forever := make(chan time.Time)
		defer close(forever)
		C = forever
	}

	select {
	case <-p.done:
		return 0, ErrClosedPipe
	default:
	}

	select {
	case bw := <-p.wrCh:
		nr := copy(b, bw)
		p.rdCh <- nr
		return nr, nil
	case <-p.done:
		return 0, ErrClosedPipe
	case <-C:
		return 0, os.ErrDeadlineExceeded
	}
}

func (p *pipe) write(b []byte) (n int, err error) {
	var C <-chan time.Time

	p.writeDeadlineMu.Lock()
	writeDeadline := p.writeDeadline
	p.writeDeadlineMu.Unlock()

	if writeDeadline != nil {
		defer func() {
			p.writeDeadlineMu.Lock()
			p.writeDeadline = nil
			p.writeDeadlineMu.Unlock()
		}()
		d := time.Until(*writeDeadline)
		if d <= 0 {
			return 0, os.ErrDeadlineExceeded
		}
		timer := time.NewTimer(d)
		C = timer.C
		defer timer.Stop()
	} else {
		forever := make(chan time.Time)
		defer close(forever)
		C = forever
	}

	select {
	case <-p.done:
		return 0, ErrClosedPipe
	default:
		p.wrMu.Lock()
		defer p.wrMu.Unlock()
	}

	for once := true; once || len(b) > 0; once = false {
		select {
		case p.wrCh <- b:
			nw := <-p.rdCh
			b = b[nw:]
			n += nw
		case <-p.done:
			return n, ErrClosedPipe
		case <-C:
			return n, os.ErrDeadlineExceeded
		}
	}
	return n, nil
}

// A PipeReader is the read half of a pipe.
type PipeReader struct{ pipe }

func (r *PipeReader) Read(data []byte) (n int, err error) {
	return r.pipe.read(data)
}

func (r *PipeReader) Close() error {
	r.pipe.doneOnce.Do(func() { close(r.pipe.done) })
	return nil
}

func (r *PipeReader) SetReadDeadline(t time.Time) error {
	r.readDeadlineMu.Lock()
	r.readDeadline = &t
	r.readDeadlineMu.Unlock()
	return nil
}

func (w *PipeWriter) SetWriteDeadline(t time.Time) error {
	w.r.pipe.writeDeadlineMu.Lock()
	w.r.pipe.writeDeadline = &t
	w.r.pipe.writeDeadlineMu.Unlock()
	return nil
}

// A PipeWriter is the write half of a pipe.
type PipeWriter struct{ r PipeReader }

func (w *PipeWriter) Write(data []byte) (n int, err error) {
	return w.r.pipe.write(data)
}

func (w *PipeWriter) Close() error {
	w.r.pipe.doneOnce.Do(func() { close(w.r.pipe.done) })
	return nil
}

// Pipe is equivalent to `io.Pipe` but with deadline support.
func Pipe() (*PipeReader, *PipeWriter) {
	pw := &PipeWriter{r: PipeReader{pipe: pipe{
		wrCh: make(chan []byte),
		rdCh: make(chan int),
		done: make(chan struct{}),
	}}}
	return &pw.r, pw
}
