// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 The Noisy Sockets Authors.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package message

import (
	"encoding/json"
	"fmt"

	"github.com/mitchellh/mapstructure"
)

const (
	// MaxSize is the maximum supported WebSocket message size.
	MaxSize = 65536
)

type Meta struct {
	// APIVersion is the version of the API.
	APIVersion string `json:"apiVersion" mapstructure:"apiVersion"`
	// Kind is the kind of the message.
	Kind string `json:"kind" mapstructure:"kind"`
	// ID is the globally unique message ID, or in the case of an Ack, the ID of
	// the message being acknowledged.
	ID string `json:"id" mapstructure:"id"`
}

func (m *Meta) GetID() string {
	return m.ID
}

func (m *Meta) SetID(id string) {
	m.ID = id
}

// Message is a message that can be sent/received.
type Message interface {
	GetAPIVersion() string
	GetKind() string
	GetID() string
	SetID(id string)
}

var knownTypes = make(map[string]func() Message)

// RegisterType registers a known message type.
func RegisterType(newMessageOfType func() Message) {
	msg := newMessageOfType()
	knownTypes[Type(msg)] = newMessageOfType
}

// Marshal marshals a message, returning the data.
func Marshal(msg Message) ([]byte, error) {
	// Convert the message to a map.
	m := make(map[string]interface{})
	if err := mapstructure.Decode(msg, &m); err != nil {
		return nil, err
	}

	// Add the message type information to the map.
	m["apiVersion"] = msg.GetAPIVersion()
	m["kind"] = msg.GetKind()

	// Marshal the map to JSON.
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Unmarshal unmarshals a message from the given data.
func Unmarshal(data []byte) (Message, error) {
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	typ := fmt.Sprintf("%s/%s", meta.APIVersion, meta.Kind)
	newMessageOfType, ok := knownTypes[typ]
	if !ok {
		return nil, fmt.Errorf("unknown message type: %s", typ)
	}

	msg := newMessageOfType()
	if err := json.Unmarshal(data, msg); err != nil {
		return nil, err
	}

	return msg, nil
}

// Type returns the string formatted type of the message.
func Type(msg Message) string {
	return fmt.Sprintf("%s/%s", msg.GetAPIVersion(), msg.GetKind())
}
