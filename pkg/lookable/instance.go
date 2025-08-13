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

// NewInstanceInfo creates an InstanceInfo from an EC2 instance
// The asgInstance parameter is optional - if provided, ASG lifecycle state will be included
func NewInstanceInfo(instance ec2types.Instance, asgInstance *asgtypes.Instance) *InstanceInfo {
	var ipv6Addr string
	if instance.Ipv6Address != nil {
		ipv6Addr = *instance.Ipv6Address
	}

	var privateIP string
	if instance.PrivateIpAddress != nil {
		privateIP = *instance.PrivateIpAddress
	}

	var stateName ec2types.InstanceStateName
	if instance.State != nil {
		stateName = instance.State.Name
	}

	var azName string
	if instance.Placement.AvailabilityZone != nil {
		azName = *instance.Placement.AvailabilityZone
	}

	instanceInfo := &InstanceInfo{
		InstanceID:       *instance.InstanceId,
		PrivateIP:        privateIP,
		IPv6Address:      ipv6Addr,
		InstanceState:    stateName,
		AvailabilityZone: azName,
		InstanceType:     string(instance.InstanceType),
		// Default values for ASG fields
		LifecycleState: "",
		HealthStatus:   "",
	}

	// Add ASG lifecycle state if available
	if asgInstance != nil {
		instanceInfo.LifecycleState = asgInstance.LifecycleState
		instanceInfo.HealthStatus = *asgInstance.HealthStatus
	}

	return instanceInfo
}

// GetIP returns the appropriate IP address based on the ipv6 flag
func (i *InstanceInfo) GetIP(ipv6 bool) string {
	if ipv6 {
		return i.IPv6Address
	}
	return i.PrivateIP
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
