package slice

func Exists(items []string, pattern string) bool {
	for _, item := range items {
		if item == pattern {
			return true
		}
	}

	return false
}
