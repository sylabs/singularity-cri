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
