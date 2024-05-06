// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package message_test

import (
	"encoding/base64"
	"testing"

	"github.com/noisysockets/shell/internal/message"
	"github.com/noisysockets/shell/internal/message/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageSerialization(t *testing.T) {
	msg := &v1alpha1.Data{
		Meta: message.Meta{
			APIVersion: v1alpha1.APIVersion,
			Kind:       "Data",
		},
		Data: base64.StdEncoding.EncodeToString([]byte("hello, world!")),
	}

	data, err := message.Marshal(msg)
	require.NoError(t, err)

	newMsg, err := message.Unmarshal(data)
	require.NoError(t, err)

	assert.Equal(t, msg, newMsg)
}
