package changes

import (
	"github.com/AirVantage/overlord/pkg/set"
)

// Changes keeps track of added/removed IPs for a Resource.
// We store IPs as strings to support both IPv4 and IPv6.
type Changes[T comparable] struct {
	addedIPs   *set.Set[T]
	removedIPs *set.Set[T]
}

// NewChanges return a pointer to an initialized Changes struct.
func New[T comparable]() *Changes[T] {
	return &Changes[T]{
		addedIPs:   set.New[T](),
		removedIPs: set.New[T](),
	}
}


// Log changes
func (c *Changes[T])Add(add T) {
	c.addedIPs.Add(add)
}
func (c *Changes[T])Remove(rem T) {
	c.removedIPs.Add(rem)
}

// Return changes as slice
func (c *Changes[T])Added() []T {
	return c.addedIPs.ToSlice()
}
func (c *Changes[T])Removed() []T {
	return c.removedIPs.ToSlice()
}

// Return a deep copy of current object
func (c *Changes[T])Copy() *Changes[T] {
	var copy *Changes[T] = New[T]()

	for _, added := range c.addedIPs.ToSlice() {
		copy.addedIPs.Add(added)
	}
	for _, removed := range c.removedIPs.ToSlice() {
		copy.removedIPs.Add(removed)
	}
	return copy
}

// NewChanges return a pointer to an initialized Changes struct.
func (c *Changes[T])Merge(m *Changes[T]) *Changes[T] {
	var merged *Changes[T] = c.Copy()

	for _, added := range m.addedIPs.ToSlice() {
		merged.addedIPs.Add(added)
	}
	for _, removed := range m.removedIPs.ToSlice() {
		merged.removedIPs.Add(removed)
	}
	return merged
}
