package lookable

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// Lookable is a group of cloud instances.
type Lookable interface {
	// LookupIPs returns the list of IP addresses of the Lookable instances, in IPv4 or IPv6.
	LookupIPs(ctx context.Context, cfg aws.Config, ipv6 bool) ([]string, error)
	String() string
}
