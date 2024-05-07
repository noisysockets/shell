// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package session

import "time"

const (
	defaultNetTimeout        = 10 * time.Second
	defaultAckTimeout        = 30 * time.Second
	defaultHeartbeatInterval = time.Minute
	defaultHeartbeatTimeout  = 30 * time.Second
)
