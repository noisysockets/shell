// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package shell_test

import (
	"testing"

	"github.com/noisysockets/shell"
	"github.com/stretchr/testify/assert"
)

func TestSafeEnvVars(t *testing.T) {
	env := []string{
		"TERM=xterm-256color",
		"LANG=en_US.UTF-8",
		"LC_ALL=C",
		"USER=example",
		"PATH=/usr/bin:/bin",
		"TZ=UTC",
		"LC_TIME=en_US.UTF-8",
		"SECRET_KEY=supersecret",
	}

	expectedEnv := []string{
		"TERM=xterm-256color",
		"LANG=en_US.UTF-8",
		"LC_ALL=C",
		"LC_TIME=en_US.UTF-8",
	}

	assert.ElementsMatch(t, expectedEnv, shell.SafeEnvVars(env))
}
