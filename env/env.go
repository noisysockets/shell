// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

// Package env provides utilities for working with environment variables.
package env

import (
	"os"
	"strings"
)

// FilterSafe filters the given client environment variables into a safe subset
// that can be passed to the server, includes TERM, LANG, and LC_*.
func FilterSafe(env []string) []string {
	var safeEnv []string
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		if key == "TERM" || key == "LANG" || strings.HasPrefix(key, "LC_") {
			safeEnv = append(safeEnv, e)
		}
	}

	return safeEnv
}

// Default returns a minimal list of env vars that will be inherited by the
// shell process. This is used to avoid leaking sensitive information.
func Default() []string {
	var env []string
	for _, key := range []string{"USER", "LOGNAME", "HOME", "PATH", "SHELL", "TZ"} {
		if os.Getenv(key) != "" {
			env = append(env, key+"="+os.Getenv(key))
		}
	}

	return env
}
