package lookable

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// AutoScalingGroup is a Lookable ASG tag name.
type AutoScalingGroup string

func (asg AutoScalingGroup) String() string {
	return string(asg)
}

// LookupIPs of all the instances in this AutoScalingGroup.
func (asg AutoScalingGroup) doLookupIPs(as ASGAPI, ec EC2API, ctx context.Context, ipv6 bool) ([]string, error) {

	var output []string

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
	for _, inst := range resp2.AutoScalingGroups[0].Instances {
		// log.Println("Got instance Id:"+*inst.InstanceId+" health:"+*inst.HealthStatus+" LifeCycle:"+string(inst.LifecycleState))
		if inst.LifecycleState == asgtypes.LifecycleStateInService ||
			inst.LifecycleState == asgtypes.LifecycleStateTerminating ||
			inst.LifecycleState == asgtypes.LifecycleStateDetaching ||
			inst.LifecycleState == asgtypes.LifecycleStateEnteringStandby {
			// log.Println("added")
			instances = append(instances, *inst.InstanceId)
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
			if ipv6 {
				output = append(output, *instance.Ipv6Address)
			} else {
				output = append(output, *instance.PrivateIpAddress)
			}
		}
	}

	return output, nil
}

// LookupIPs of all the instances in this AutoScalingGroup.
func (asg AutoScalingGroup) LookupIPs(ctx context.Context, cfg aws.Config, ipv6 bool) ([]string, error) {
	return asg.doLookupIPs(autoscaling.NewFromConfig(cfg), ec2.NewFromConfig(cfg), ctx, ipv6)
}
