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
