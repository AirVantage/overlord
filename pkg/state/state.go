package state

import (
	"github.com/AirVantage/overlord/pkg/resource"
	"github.com/AirVantage/overlord/pkg/set"
)


type State struct {
	Ipsets map[string]*set.Set[string]
	Templates map[string]*resource.Resource
}

// NewChanges return a pointer to an initialized Changes struct.
func New() *State {
	return &State{
		Ipsets: make(map[string]*set.Set[string]),
		Templates: make(map[string]*resource.Resource),
	}
}

