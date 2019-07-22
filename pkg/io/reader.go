//  Copyright (c) 2018-2019 Sylabs, Inc. All rights reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package io

import (
	"context"
	"io"
)

// ContextReader wraps an io.Reader to make it respect the given Context.
// If there is a blocking read, ContextReader will return
// whenever the context is cancelled (the return values are n=0
// and err=ctx.Err() in that case).
//
// Note: this wrapper DOES NOT ACTUALLY cancel the underlying
// write – there is no way to do that with the standard go io
// interface. So the read will happen or hang. So, use
// this sparingly, make sure to cancel the read as necessary
// (e.g. closing a connection whose context is up, etc.).
//
// Furthermore, in order to protect your memory from being read
// before you've cancelled the context, this io.Reader will
// allocate a buffer of the same size, and **copy** into the client's
// if the read succeeds in time.
type ContextReader struct {
	r   io.Reader
	ctx context.Context
}

// NewContextReader will return a new ContextReader.
func NewContextReader(ctx context.Context, r io.Reader) *ContextReader {
	return &ContextReader{
		r:   r,
		ctx: ctx,
	}
}

type ioret struct {
	n   int
	err error
}

func (r *ContextReader) Read(p []byte) (int, error) {
	buf := make([]byte, len(p))

	c := make(chan ioret, 1)

	go func() {
		n, err := r.r.Read(buf)
		c <- ioret{n, err}
		close(c)
	}()

	select {
	case ret := <-c:
		copy(p, buf)
		return ret.n, ret.err
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	}
}
