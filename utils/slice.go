package utils

// ReverseSlice reverses the given original slice
func ReverseSlice(labels []string) {
	for i, j := 0, len(labels)-1; i < j; i, j = i+1, j-1 {
		labels[i], labels[j] = labels[j], labels[i]
	}
}
