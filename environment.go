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
	"os"
	"strings"
)

// SafeEnvVars filters the os.Environ() into a safe subset that can be passed
// to the remote shell server, includes TERM, LANG, and LC_*.
func SafeEnvVars(env []string) []string {
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

// defaultEnv returns a minimal list of env vars that can be inherited by the
// shell process. This is used to avoid leaking sensitive information.
func defaultEnv() []string {
	var env []string
	for _, key := range []string{"USER", "LOGNAME", "HOME", "PATH", "SHELL", "TZ"} {
		if os.Getenv(key) != "" {
			env = append(env, key+"="+os.Getenv(key))
		}
	}

	return env
}
