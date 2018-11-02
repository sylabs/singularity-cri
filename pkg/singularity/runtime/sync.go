package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
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
// and passes them to the channel. ObserveState creates socket
// if necessary. Since this function is used to sync with runtime the
// returned channel is unbuffered. The channel will be closed if either
// container has transmitted into StateExited or any error during networking occurred.
// ObserveState returns error only if it fails to start listener on the passed socket.
func ObserveState(ctx context.Context, socket string) (<-chan State, error) {
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("could not listen sync socket: %v", err)
	}

	syncChan := make(chan State)
	go func() {
		defer close(syncChan)
		defer ln.Close()

		for {
			select {
			case <-ctx.Done():
				log.Printf("context is done")
				return
			case conn := <-nextConn(ln):
				if conn == nil {
					log.Printf("could not accept sync socket connection")
					return
				}
				state, err := readState(conn)
				if err != nil {
					log.Printf("could not read state at %s: %v", socket, err)
					return
				}
				log.Printf("received state %d at %s", state, socket)
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

	var state State
	switch status.Status {
	case "creating":
		state = StateCreating
	case "created":
		state = StateCreated
	case "running":
		state = StateRunning
	case "stopped":
		state = StateExited
	default:
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
			log.Printf("accept failed: %v", err)
			return
		}
		next <- conn
	}()
	return next
}
