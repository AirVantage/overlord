package lookable

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// InstanceInfo contains detailed information about an instance
type InstanceInfo struct {
	InstanceID       string
	PrivateIP        string
	IPv6Address      string
	LifecycleState   asgtypes.LifecycleState
	HealthStatus     string
	InstanceState    ec2types.InstanceStateName
	AvailabilityZone string
	InstanceType     string
}

// GetIP returns the appropriate IP address based on the ipv6 flag
func (i *InstanceInfo) GetIP(ipv6 bool) string {
	if ipv6 {
		return i.IPv6Address
	}
	return i.PrivateIP
}

// IsHealthy returns true if the instance is in a healthy state
func (i *InstanceInfo) IsHealthy() bool {
	return validLifecycleStates[i.LifecycleState]
}

// GetHash returns a hash of the instance state for change detection
func (i *InstanceInfo) GetHash() string {
	// Create a string representation of non static part of the instance state
	stateStr := fmt.Sprintf("%s:%s:%s",
		i.LifecycleState,
		i.HealthStatus,
		i.InstanceState,
	)

	hash := sha256.Sum256([]byte(stateStr))
	return hex.EncodeToString(hash[:])
}

// Equals returns true if two instances have the same state
func (i *InstanceInfo) Equals(other *InstanceInfo) bool {
	if other == nil {
		return false
	}
	return i.GetHash() == other.GetHash()
}
