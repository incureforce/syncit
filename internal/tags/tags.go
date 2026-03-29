package tags

// Equal reports whether a and b are the same ordered tag list.
func Equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Subset reports whether every element of fileTags appears in clientTags (file_tags ⊆ client_tags).
func Subset(fileTags, clientTags []string) bool {
	if len(fileTags) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(clientTags))
	for _, t := range clientTags {
		set[t] = struct{}{}
	}
	for _, t := range fileTags {
		if _, ok := set[t]; !ok {
			return false
		}
	}
	return true
}
