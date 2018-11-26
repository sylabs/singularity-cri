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

package truncindex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Test the behavior of TruncIndex, an index for querying IDs from a non-conflicting prefix.
func TestTruncIndex(t *testing.T) {
	index := NewTruncIndex(64)
	if _, err := index.Get("foobar"); err == nil {
		require.Equal(t, ErrNotFound, err)
	}

	if err := index.Add("I have a space", struct{}{}); err == nil {
		require.Equal(t, ErrIllegalChar, err)
	}

	id := "99b36c2c326ccc11e726eee6ee78a0baf166ef96"
	if err := index.Add(id, id); err != nil {
		require.NoError(t, err)
	}

	if err := index.Add("", struct{}{}); err == nil {
		require.Equal(t, ErrEmptyPrefix, err)
	}

	assertIndexGet(t, index, "abracadabra", nil, ErrNotFound)
	assertIndexGet(t, index, "", nil, ErrEmptyPrefix)
	assertIndexGet(t, index, id, id, nil)
	assertIndexGet(t, index, id[:1], id, nil)
	assertIndexGet(t, index, id[:len(id)/2], id, nil)
	assertIndexGet(t, index, id[len(id)/2:], nil, ErrNotFound)

	id2 := id[:6] + "blabla"
	if err := index.Add(id2, id2); err != nil {
		require.NoError(t, err)
	}
	assertIndexGet(t, index, id, id, nil)
	assertIndexGet(t, index, id2, id2, nil)

	assertIndexGet(t, index, id[:6], nil, ErrAmbiguousPrefix{id[:6]})
	assertIndexGet(t, index, id[:4], nil, ErrAmbiguousPrefix{id[:4]})
	assertIndexGet(t, index, id[:1], nil, ErrAmbiguousPrefix{id[:1]})
	assertIndexGet(t, index, id[:7], id, nil)
	assertIndexGet(t, index, id2[:7], id2, nil)

	err := index.Delete("non-existing")
	require.Equal(t, ErrNotFound, err)

	err = index.Delete("")
	require.Equal(t, ErrEmptyPrefix, err)

	err = index.Delete(id2)
	require.NoError(t, err)

	assertIndexGet(t, index, id2, nil, ErrNotFound)
	assertIndexGet(t, index, id2[:7], nil, ErrNotFound)
	assertIndexGet(t, index, id2[:11], nil, ErrNotFound)

	assertIndexGet(t, index, id[:6], id, nil)
	assertIndexGet(t, index, id[:4], id, nil)
	assertIndexGet(t, index, id[:1], id, nil)

	assertIndexGet(t, index, id[:7], id, nil)
	assertIndexGet(t, index, id[:15], id, nil)
	assertIndexGet(t, index, id, id, nil)

	assertIndexIterate(t)
	assertIndexIterateDoNotPanic(t)
}

func assertIndexIterate(t *testing.T) {
	ids := []string{
		"19b36c2c326ccc11e726eee6ee78a0baf166ef96",
		"28b36c2c326ccc11e726eee6ee78a0baf166ef96",
		"37b36c2c326ccc11e726eee6ee78a0baf166ef96",
		"46b36c2c326ccc11e726eee6ee78a0baf166ef96",
	}

	index := NewTruncIndex(64)
	for _, id := range ids {
		index.Add(id, struct{}{})
	}

	index.Iterate(func(key string, item interface{}) {
		for _, id := range ids {
			if key == id {
				return
			}
		}

		t.Fatalf("An unknown ID '%s'", key)
	})
}

func assertIndexIterateDoNotPanic(t *testing.T) {
	ids := []string{
		"19b36c2c326ccc11e726eee6ee78a0baf166ef96",
		"28b36c2c326ccc11e726eee6ee78a0baf166ef96",
	}

	index := NewTruncIndex(64)
	for _, id := range ids {
		index.Add(id, struct{}{})
	}

	iterationStarted := make(chan bool, 1)
	go func() {
		<-iterationStarted
		index.Delete("19b36c2c326ccc11e726eee6ee78a0baf166ef96")
	}()

	index.Iterate(func(key string, item interface{}) {
		if key == "19b36c2c326ccc11e726eee6ee78a0baf166ef96" {
			iterationStarted <- true
			time.Sleep(100 * time.Millisecond)
		}
	})
}

func assertIndexGet(t *testing.T, index *TruncIndex, key string, expectedResult interface{}, expectError error) {
	result, err := index.Get(key)
	require.Equal(t, expectError, err)
	require.Equal(t, expectedResult, result)
}
