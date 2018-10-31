package kube

import (
	"context"
	"encoding/json"
	"log"
	"net"
)

// State defines type for manipulating with container's state.
type State int

const (
	// StateCreating means container is being created at the moment.
	StateCreating = State(1 + iota)
	// StateCreated means container is created without any errors.
	StateCreated
	// StateRunning means container is running at the moment.
	StateRunning
	// StateExited means container has finished possibly with errors.
	StateExited
)

// SyncWithRuntime listens on passed socket for container state changes
// and passes them to the channel. SyncWithRuntime creates socket
// if necessary. Since this function is used to sync with runtime the
// returned channel is unbuffered. The channel will be closed if either
// any error during decoding receiving state occurs or container has transmitted into StateExited.
// This function blocks until runtime connects to socket for writing.
func SyncWithRuntime(ctx context.Context, socket string) <-chan State {
	type statusInfo struct {
		Status string `json:"status"`
	}

	syncChan := make(chan State)
	go func() {
		defer close(syncChan)

		ln, err := net.Listen("unix", socket)
		if err != nil {
			log.Printf("could not listen sync socket: %v", err)
			return

		}
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("could not accept sync socket connection: %v", err)
			return
		}

		defer conn.Close()

		dec := json.NewDecoder(conn)
		var status statusInfo

		for {
			select {
			case <-ctx.Done():
				return
			case dec.More():
				err := dec.Decode(&status)
				if err != nil {
					log.Printf("could not read state: %v", err)
					return
				}
				switch status.Status {
				case "creating":
					syncChan <- StateCreating
				case "created":
					syncChan <- StateCreated
				case "running":
					syncChan <- StateRunning
				case "stopped":
					syncChan <- StateExited
					return
				default:
					log.Printf("unknown status received on %s: %s", socket, status.Status)
				}
			}
		}
	}()
	return syncChan
}
