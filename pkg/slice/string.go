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

package slice

// MergeString takes two string slices and merges them making sure
// no duplicates appear in resulting slice.
func MergeString(a []string, v ...string) []string {
	unique := make(map[string]struct{})
	for _, tag := range append(a, v...) {
		unique[tag] = struct{}{}
	}
	merged := make([]string, 0, len(unique))
	for str := range unique {
		merged = append(merged, str)
	}
	return merged
}

// RemoveFromString returns passed slice without first occurrence of element v.
// It does not make a copy of a passed slice.
func RemoveFromString(a []string, v string) []string {
	for i, str := range a {
		if str == v {
			return append(a[:i], a[i+1:]...)
		}
	}
	return a
}
