package state

import (
	"github.com/AirVantage/overlord/pkg/lookable"
	"github.com/AirVantage/overlord/pkg/resource"
	"github.com/AirVantage/overlord/pkg/set"
)

type State struct {
	Ipsets         map[string]*set.Set[string]
	InstanceSets   map[string][]*lookable.InstanceInfo
	InstanceHashes map[string]map[string]string // group -> instanceID -> hash
	Templates      map[string]*resource.Resource
}

// NewChanges return a pointer to an initialized Changes struct.
func New() *State {
	return &State{
		Ipsets:         make(map[string]*set.Set[string]),
		InstanceSets:   make(map[string][]*lookable.InstanceInfo),
		InstanceHashes: make(map[string]map[string]string),
		Templates:      make(map[string]*resource.Resource),
	}
}
