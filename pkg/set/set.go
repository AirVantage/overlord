package set

// Generic set data structure

// Strings is a set of unique strings.
type Set[T comparable] map[T]struct{}

// NewStringSet instantiates a new generic Set .
func New[T comparable]() *Set[T] {
	s := make(Set[T])
	return &s
}

// Add a string to the set.
func (ss Set[T]) Add(s T) {
	if _, exists := ss[s]; !exists {
		ss[s] = struct{}{}
	}
}

// Has returns true if a strings is part of the set.
func (ss Set[T]) Has(s T) bool {
	_, exists := ss[s]
	return exists
}

// ToSlice returns a copy the set as a slice of strings.
func (ss Set[T]) ToSlice() []T {
	// Allocate a large enough slice
	slice := make([]T, 0, len(ss))

	for key, _ := range ss {
		slice = append(slice, key)
	}
	return slice
}
