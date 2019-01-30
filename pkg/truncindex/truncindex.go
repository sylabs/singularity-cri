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

// The original implementation of truncindex allowed string identifiers
// storing only. That is why a couple of changes were made to satisfy
// Singularity needs. One major change is that not only strings but any
// arbitrary data can be stored and fetched from TruncIndex now. Also the
// way prefix is tested against spaces content is optimised.

package truncindex

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tchap/go-patricia/patricia"
)

var (
	// ErrEmptyPrefix is returned when key or prefix is empty.
	ErrEmptyPrefix = errors.New("empty prefix not allowed")

	// ErrIllegalChar is returned when key contains spaces.
	ErrIllegalChar = errors.New("illegal character: ' '")

	// ErrNotFound is returned when item is not found in trie.
	ErrNotFound = errors.New("item not found")

	// ErrAlreadyExists is returned when key is already present in index.
	ErrAlreadyExists = errors.New("already exists")
)

// ErrAmbiguousPrefix is returned if the prefix was ambiguous
// (multiple keys for the prefix).
type ErrAmbiguousPrefix struct {
	prefix string
}

func (e ErrAmbiguousPrefix) Error() string {
	return fmt.Sprintf("multiple items found for provided prefix: %s", e.prefix)
}

// TruncIndex allows the retrieval of items by associated key or any of it unique prefixes.
type TruncIndex struct {
	sync.RWMutex
	trie *patricia.Trie
	keys map[string]struct{}
}

// NewTruncIndex creates a new TruncIndex and initializes with a list of IDs.
// Retured index is tread-safe to use.
func NewTruncIndex(maxKeyLen int) *TruncIndex {
	return &TruncIndex{
		keys: make(map[string]struct{}),
		trie: patricia.NewTrie(patricia.MaxPrefixPerNode(maxKeyLen)),
	}
}

// Add adds a new key-item pair to the TruncIndex.
func (idx *TruncIndex) Add(key string, item interface{}) error {
	if key == "" {
		return ErrEmptyPrefix
	}
	if strings.IndexByte(key, ' ') != -1 {
		return ErrIllegalChar
	}

	idx.Lock()
	defer idx.Unlock()
	if _, exists := idx.keys[key]; exists {
		return ErrAlreadyExists
	}
	if inserted := idx.trie.Insert(patricia.Prefix(key), item); !inserted {
		return fmt.Errorf("could not insert item for key %q", key)
	}
	idx.keys[key] = struct{}{}
	return nil
}

// Delete removes kay and associated item from the TruncIndex.
// If there are multiple IDs with the given prefix, an error is returned.
func (idx *TruncIndex) Delete(key string) error {
	if key == "" {
		return ErrEmptyPrefix
	}
	if strings.IndexByte(key, ' ') != -1 {
		return ErrIllegalChar
	}

	idx.Lock()
	defer idx.Unlock()
	if _, exists := idx.keys[key]; !exists {
		return ErrNotFound
	}
	if deleted := idx.trie.Delete(patricia.Prefix(key)); !deleted {
		return fmt.Errorf("could not remove item for key %q", key)
	}
	delete(idx.keys, key)
	return nil
}

// Get retrieves an item from the TruncIndex by key or its prefix.
// If there are multiple keys with the given prefix, an error is returned.
func (idx *TruncIndex) Get(key string) (interface{}, error) {
	if key == "" {
		return nil, ErrEmptyPrefix
	}
	if strings.IndexByte(key, ' ') != -1 {
		return nil, ErrIllegalChar
	}

	var found interface{}
	findByKey := func(prefix patricia.Prefix, item patricia.Item) error {
		if found != nil {
			return ErrAmbiguousPrefix{prefix: key}
		}
		found = item
		return nil
	}

	idx.RLock()
	defer idx.RUnlock()
	if err := idx.trie.VisitSubtree(patricia.Prefix(key), findByKey); err != nil {
		return nil, err
	}
	if found != nil {
		return found, nil
	}
	return nil, ErrNotFound
}

// Iterate iterates over all stored items and passes each of them to the given
// handler. Take care that the handler method does not call any public
// method on truncindex as the internal locking is not reentrant/recursive
// and will result in deadlock.
func (idx *TruncIndex) Iterate(handler func(key string, item interface{})) {
	idx.RLock()
	defer idx.RUnlock()
	idx.trie.Visit(func(prefix patricia.Prefix, item patricia.Item) error {
		handler(string(prefix), item)
		return nil
	})
}
