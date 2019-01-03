// Copyright (c) 2018 Sylabs, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtime

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObserveState_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	socket := filepath.Join(os.TempDir(), fmt.Sprintf("cri-test-%s.sock", t.Name()))

	state, err := ObserveState(ctx, socket)
	require.NoError(t, err, "could not listen on socket")
	cancel()
	assert.Equal(t, StateUnknown, <-state)
	assert.True(t, os.IsNotExist(os.Remove(socket)))
}

func TestObserveState_InvalidJSON(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	socket := filepath.Join(os.TempDir(), fmt.Sprintf("cri-test-%s.sock", t.Name()))

	state, err := ObserveState(ctx, socket)
	require.NoError(t, err, "could not listen on socket")
	go func(t *testing.T) {
		c, err := net.Dial("unix", socket)
		require.NoError(t, err)
		_, err = c.Write([]byte(`{"this": is [invalid json}`))
		assert.NoError(t, err)
		assert.NoError(t, c.Close())
	}(t)

	assert.Equal(t, StateUnknown, <-state)
	cancel()
	assert.True(t, os.IsNotExist(os.Remove(socket)))
}

func TestObserveState_AllWithDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	socket := filepath.Join(os.TempDir(), fmt.Sprintf("cri-test-%s.sock", t.Name()))

	state, err := ObserveState(ctx, socket)
	require.NoError(t, err, "could not listen on socket")
	go func(t *testing.T) {
		c, err := net.Dial("unix", socket)
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
		_, err = c.Write([]byte(`{"status": "creating"}`))
		assert.NoError(t, err)
		assert.NoError(t, c.Close())
		time.Sleep(time.Millisecond * 5)

		c, err = net.Dial("unix", socket)
		require.NoError(t, err)
		time.Sleep(time.Millisecond * 3)
		_, err = c.Write([]byte(`{"status": "created"}`))
		assert.NoError(t, err)
		assert.NoError(t, c.Close())

		time.Sleep(time.Millisecond * 5)
		c, err = net.Dial("unix", socket)
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
		_, err = c.Write([]byte(`{"status": "running"}`))
		assert.NoError(t, err)
		assert.NoError(t, c.Close())

		time.Sleep(time.Millisecond * 20)
		c, err = net.Dial("unix", socket)
		require.NoError(t, err)
		time.Sleep(time.Millisecond * 5)
		_, err = c.Write([]byte(`{"status": "stopped"}`))
		assert.NoError(t, err)
		assert.NoError(t, c.Close())

	}(t)

	assert.Equal(t, StateCreating, <-state)
	assert.Equal(t, StateCreated, <-state)
	assert.Equal(t, StateRunning, <-state)
	assert.Equal(t, StateExited, <-state)
	assert.Equal(t, StateUnknown, <-state)
	cancel()
	assert.True(t, os.IsNotExist(os.Remove(socket)))
}
