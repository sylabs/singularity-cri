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

package rand

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateID returns unique random id of passed length generated with crypto/rand.
func GenerateID(len int) string {
	buf := make([]byte, (len-1)/2+1)
	rand.Read(buf)
	return hex.EncodeToString(buf)[:len]
}
