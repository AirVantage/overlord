package set

// Strings is a set of unique strings.
type Strings map[string]struct{}

// NewStringSet instantiates a new set of strings.
func NewStringSet() Strings {
	return make(Strings)
}

// Add a string to the set.
func (ss Strings) Add(s string) {
	if _, exists := ss[s]; !exists {
		ss[s] = struct{}{}
	}
}

// Has returns true if a strings is part of the set.
func (ss Strings) Has(s string) bool {
	_, exists := ss[s]
	return exists
}

// ToSlice returns a copy the set as a slice of strings.
func (ss Strings) ToSlice() []string {
	slice := make([]string, len(ss))
	i := 0
	for key := range ss {
		slice[i] = key
		i++
	}
	return slice
}
