package lookable

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var validLifecycleStates = map[asgtypes.LifecycleState]bool{
	asgtypes.LifecycleStatePending:            false,
	asgtypes.LifecycleStatePendingWait:        false,
	asgtypes.LifecycleStatePendingProceed:     false,
	asgtypes.LifecycleStateInService:          true,
	asgtypes.LifecycleStateTerminating:        true,
	asgtypes.LifecycleStateTerminatingWait:    true,
	asgtypes.LifecycleStateTerminatingProceed: false,
	asgtypes.LifecycleStateTerminated:         false,
	asgtypes.LifecycleStateDetaching:          true,
	asgtypes.LifecycleStateDetached:           false,
	asgtypes.LifecycleStateEnteringStandby:    true,
	asgtypes.LifecycleStateStandby:            false,
	// Note: warmed pool not handled
}

// AutoScalingGroup is a Lookable ASG tag name.
type AutoScalingGroup string

func (asg AutoScalingGroup) String() string {
	return string(asg)
}

// LookupIPs of all the instances in this AutoScalingGroup.
func (asg AutoScalingGroup) doLookupIPs(as ASGAPI, ec EC2API, ctx context.Context, ipv6 bool) ([]string, error) {
	instances, err := asg.doLookupInstances(as, ec, ctx)
	if err != nil {
		return nil, err
	}

	var output []string
	for _, instance := range instances {
		output = append(output, instance.GetIP(ipv6))
	}

	return output, nil
}

// doLookupInstances returns detailed information about all instances in this AutoScalingGroup.
func (asg AutoScalingGroup) doLookupInstances(as ASGAPI, ec EC2API, ctx context.Context) ([]*InstanceInfo, error) {
	var output []*InstanceInfo

	// Find the ASG instances
	params2 := &autoscaling.DescribeAutoScalingGroupsInput{
		Filters: []asgtypes.Filter{
			{
				// FIXME: Should we use "tag:Name" to match other filters ?
				Name:   aws.String("tag-value"),
				Values: []string{asg.String()},
			},
		},
	}
	resp2, err := as.DescribeAutoScalingGroups(ctx, params2)
	if err != nil {
		return nil, err
	}
	if len(resp2.AutoScalingGroups) == 0 {
		return output, nil
	}

	numInstances := len(resp2.AutoScalingGroups[0].Instances)
	if numInstances == 0 {
		return output, nil
	}

	// Make a list of healthy instance ID in the ASG
	instances := make([]string, 0, numInstances)
	instanceDetails := make(map[string]*asgtypes.Instance)
	for _, inst := range resp2.AutoScalingGroups[0].Instances {
		//log.Println("Got instance Id:" + *inst.InstanceId + " health:" + *inst.HealthStatus + " LifeCycle:" + string(inst.LifecycleState))
		if validLifecycleStates[inst.LifecycleState] {
			// log.Println("added")
			instances = append(instances, *inst.InstanceId)
			instanceDetails[*inst.InstanceId] = &inst
		}
	}

	// No healthy instances
	if len(instances) == 0 {
		return output, nil
	}

	// Find running instances IP
	params3 := &ec2.DescribeInstancesInput{
		InstanceIds: instances,
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{string(ec2types.InstanceStateNameRunning)},
			},
		},
	}
	resp3, err := ec.DescribeInstances(ctx, params3)
	if err != nil {
		return nil, err
	}

	for _, reservation := range resp3.Reservations {
		for _, instance := range reservation.Instances {
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
			}

			asgInstance := instanceDetails[*instance.InstanceId]
			if asgInstance != nil {
				instanceInfo.LifecycleState = asgInstance.LifecycleState
				instanceInfo.HealthStatus = *asgInstance.HealthStatus
			}

			output = append(output, instanceInfo)
		}
	}

	return output, nil
}

// LookupIPs of all the instances in this AutoScalingGroup.
func (asg AutoScalingGroup) LookupIPs(ctx context.Context, cfg aws.Config, ipv6 bool) ([]string, error) {
	return asg.doLookupIPs(autoscaling.NewFromConfig(cfg), ec2.NewFromConfig(cfg), ctx, ipv6)
}

// LookupInstances returns detailed information about all instances in this AutoScalingGroup.
func (asg AutoScalingGroup) LookupInstances(ctx context.Context, cfg aws.Config) ([]*InstanceInfo, error) {
	return asg.doLookupInstances(autoscaling.NewFromConfig(cfg), ec2.NewFromConfig(cfg), ctx)
}
