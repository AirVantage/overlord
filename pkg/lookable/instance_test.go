package lookable

import (
	"testing"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestInstanceInfo_GetHash(t *testing.T) {
	instance1 := &InstanceInfo{
		InstanceID:       "i-1234567890abcdef0",
		PrivateIP:        "10.0.1.100",
		IPv6Address:      "2001:db8::1",
		LifecycleState:   asgtypes.LifecycleStateInService,
		HealthStatus:     "Healthy",
		InstanceState:    ec2types.InstanceStateNameRunning,
		AvailabilityZone: "us-west-2a",
		InstanceType:     "t3.micro",
	}

	instance2 := &InstanceInfo{
		InstanceID:       "i-1234567890abcdef0",
		PrivateIP:        "10.0.1.100",
		IPv6Address:      "2001:db8::1",
		LifecycleState:   asgtypes.LifecycleStateTerminating, // Different state
		HealthStatus:     "Healthy",
		InstanceState:    ec2types.InstanceStateNameRunning,
		AvailabilityZone: "us-west-2a",
		InstanceType:     "t3.micro",
	}

	hash1 := instance1.GetHash()
	hash2 := instance2.GetHash()

	if hash1 == hash2 {
		t.Errorf("Expected different hashes for different lifecycle states, got same hash: %s", hash1)
	}

	// Same instance should have same hash
	instance1Copy := &InstanceInfo{
		InstanceID:       "i-1234567890abcdef0",
		PrivateIP:        "10.0.1.100",
		IPv6Address:      "2001:db8::1",
		LifecycleState:   asgtypes.LifecycleStateInService,
		HealthStatus:     "Healthy",
		InstanceState:    ec2types.InstanceStateNameRunning,
		AvailabilityZone: "us-west-2a",
		InstanceType:     "t3.micro",
	}

	hash1Copy := instance1Copy.GetHash()
	if hash1 != hash1Copy {
		t.Errorf("Expected same hash for identical instances, got different: %s vs %s", hash1, hash1Copy)
	}
}

func TestInstanceInfo_Equals(t *testing.T) {
	instance1 := &InstanceInfo{
		InstanceID:       "i-1234567890abcdef0",
		PrivateIP:        "10.0.1.100",
		IPv6Address:      "2001:db8::1",
		LifecycleState:   asgtypes.LifecycleStateInService,
		HealthStatus:     "Healthy",
		InstanceState:    ec2types.InstanceStateNameRunning,
		AvailabilityZone: "us-west-2a",
		InstanceType:     "t3.micro",
	}

	instance2 := &InstanceInfo{
		InstanceID:       "i-1234567890abcdef0",
		PrivateIP:        "10.0.1.100",
		IPv6Address:      "2001:db8::1",
		LifecycleState:   asgtypes.LifecycleStateTerminating, // Different state
		HealthStatus:     "Healthy",
		InstanceState:    ec2types.InstanceStateNameRunning,
		AvailabilityZone: "us-west-2a",
		InstanceType:     "t3.micro",
	}

	if instance1.Equals(instance2) {
		t.Error("Expected instances with different lifecycle states to not be equal")
	}

	if !instance1.Equals(instance1) {
		t.Error("Expected instance to be equal to itself")
	}

	if instance1.Equals(nil) {
		t.Error("Expected instance to not be equal to nil")
	}
}

func TestInstanceInfo_GetIP(t *testing.T) {
	instance := &InstanceInfo{
		InstanceID:       "i-1234567890abcdef0",
		PrivateIP:        "10.0.1.100",
		IPv6Address:      "2001:db8::1",
		LifecycleState:   asgtypes.LifecycleStateInService,
		HealthStatus:     "Healthy",
		InstanceState:    ec2types.InstanceStateNameRunning,
		AvailabilityZone: "us-west-2a",
		InstanceType:     "t3.micro",
	}

	// Test IPv4
	ipv4 := instance.GetIP(false)
	if ipv4 != "10.0.1.100" {
		t.Errorf("Expected IPv4 address 10.0.1.100, got %s", ipv4)
	}

	// Test IPv6
	ipv6 := instance.GetIP(true)
	if ipv6 != "2001:db8::1" {
		t.Errorf("Expected IPv6 address 2001:db8::1, got %s", ipv6)
	}
}
