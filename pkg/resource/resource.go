package resource

// Configuration file structure

import (
	"os"

	"github.com/AirVantage/overlord/pkg/lookable"
)

// ResourceConfig map the toml configuration file
type ResourceConfig struct {
	Resource Resource `toml:"template"`
}

// Resource represents a template resource that needs to be managed
type Resource struct {
	Src       string
	Dest      string
	Groups    []lookable.AutoScalingGroup
	Tags      []lookable.Tag
	Subnets   []lookable.Subnet
	ReloadCmd string `toml:"reload_cmd"`
	SrcFSInfo os.FileInfo
}
