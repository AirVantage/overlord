package lookable

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// AutoScalingGroup is a Lookable ASG tag name.
type AutoScalingGroup string

func (asg AutoScalingGroup) String() string {
	return string(asg)
}

// LookupIPs of all the instances in this AutoScalingGroup.
func (asg AutoScalingGroup) LookupIPs(ipv6 bool) ([]string, error) {
	sess := session.Must(session.NewSession())
	as := autoscaling.New(sess)
	var output []string

	// Find the ASG id
	params1 := &autoscaling.DescribeTagsInput{
		Filters: []*autoscaling.Filter{
			{
				Name:   aws.String("Value"),
				Values: []*string{aws.String(asg.String())},
			},
		},
	}
	resp1, err := as.DescribeTags(params1)
	if err != nil {
		return nil, err
	}

	asgID := resp1.Tags[0].ResourceId
	if asgID == nil {
		return nil, fmt.Errorf("could not find ASG '%s'", asg.String())
	}

	// Find the ASG instances
	params2 := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{asgID},
	}
	resp2, err := as.DescribeAutoScalingGroups(params2)
	if err != nil {
		return nil, err
	}
	if len(resp2.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("could not find ASG with tag: '%s'", asg.String())
	}

	numInstances := len(resp2.AutoScalingGroups[0].Instances)
	if numInstances == 0 {
		return output, nil
	}

	// Make a list of instance ID
	instances := make([]*string, numInstances)
	for i, inst := range resp2.AutoScalingGroups[0].Instances {
		instances[i] = inst.InstanceId
	}

	// Find running instances IP
	params3 := &ec2.DescribeInstancesInput{
		InstanceIds: instances,
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []*string{aws.String(ec2.InstanceStateNameRunning)},
			},
		},
	}
	resp3, err := ec2.New(sess).DescribeInstances(params3)
	if err != nil {
		return nil, err
	}

	for _, reservation := range resp3.Reservations {
		for _, instance := range reservation.Instances {
			if ipv6 {
				output = append(output, *instance.NetworkInterfaces[0].Ipv6Addresses[0].Ipv6Address)
			} else {
				output = append(output, *instance.PrivateIpAddress)
			}
		}
	}

	return output, nil
}
