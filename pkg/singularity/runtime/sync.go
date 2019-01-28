// Copyright (c) 2018-2019 Sylabs, Inc. All rights reserved.
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
	"encoding/json"
	"fmt"
	"net"

	"github.com/golang/glog"
)

// State defines type for manipulating with container's state.
type State int

const (
	// StateUnknown means current state is unknown (perhaps, something went wrong)
	StateUnknown State = iota
	// StateCreating means container is being created at the moment.
	StateCreating
	// StateCreated means container is created without any errors.
	StateCreated
	// StateRunning means container is running at the moment.
	StateRunning
	// StateExited means container has finished possibly with errors.
	StateExited
)

// ObserveState listens on passed socket for container state changes
// and passes them to the channel. ObserveState creates socket if necessary.
// The returned channel is buffered to eliminate any goroutine leaks.
// The channel will be closed if either container has transmitted into
// StateExited or any error during networking occurred. ObserveState returns
// error only if it fails to start listener on the passed socket.
func ObserveState(ctx context.Context, socket string) (<-chan State, error) {
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("could not listen sync socket: %v", err)
	}

	syncChan := make(chan State, 4)
	go func() {
		defer close(syncChan)
		defer ln.Close()

		for {
			select {
			case <-ctx.Done():
				glog.V(8).Infof("Context is done")
				return
			case conn := <-nextConn(ln):
				if conn == nil {
					glog.Errorf("Could not accept sync socket connection")
					return
				}
				state, err := readState(conn)
				if err != nil {
					glog.Errorf("Could not read state at %s: %v", socket, err)
					return
				}
				glog.V(4).Infof("Received state %d at %s", state, socket)
				syncChan <- state
				if state == StateExited {
					return
				}
			}
		}
	}()
	return syncChan, nil
}

func readState(conn net.Conn) (State, error) {
	type statusInfo struct {
		Status string `json:"status"`
	}

	defer conn.Close()
	dec := json.NewDecoder(conn)
	var status statusInfo
	err := dec.Decode(&status)
	if err != nil {
		return 0, fmt.Errorf("could not read state: %v", err)
	}

	state := StatusToState(status.Status)
	if state == StateUnknown {
		return 0, fmt.Errorf("received unknown status: %s", status.Status)
	}
	return state, nil
}

func nextConn(ln net.Listener) <-chan net.Conn {
	next := make(chan net.Conn)

	go func() {
		defer close(next)
		conn, err := ln.Accept()
		if err != nil {
			glog.Errorf("Accept failed: %v", err)
			return
		}
		next <- conn
	}()
	return next
}

// StatusToState is a helper func to convert container OCI status to State.
func StatusToState(status string) State {
	var state State
	switch status {
	case "creating":
		state = StateCreating
	case "created":
		state = StateCreated
	case "running":
		state = StateRunning
	case "stopped":
		state = StateExited
	}
	return state
}
