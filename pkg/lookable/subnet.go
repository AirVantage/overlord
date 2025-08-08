package lookable

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Subnet is a Lookable AWS subnet tag name.
type Subnet string

func (s Subnet) String() string {
	return string(s)
}

// LookupIPs of all the instances belonging to the given subnet.
func (s Subnet) doLookupIPs(api EC2API, ctx context.Context, ipv6 bool) ([]string, error) {
	instances, err := s.doLookupInstances(api, ctx)
	if err != nil {
		return nil, err
	}

	var output []string
	for _, instance := range instances {
		output = append(output, instance.GetIP(ipv6))
	}

	return output, nil
}

// doLookupInstances returns detailed information about all instances in the given subnet.
func (s Subnet) doLookupInstances(api EC2API, ctx context.Context) ([]*InstanceInfo, error) {
	var output []*InstanceInfo

	// Find the subnet
	params1 := &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{s.String()},
			},
		},
	}

	resp1, err := api.DescribeSubnets(ctx, params1)
	if err != nil {
		return nil, err
	}

	if len(resp1.Subnets) == 0 {
		return output, nil
	}

	// Find the running instances
	params2 := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("subnet-id"),
				Values: []string{*resp1.Subnets[0].SubnetId},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{string(types.InstanceStateNameRunning)},
			},
		},
	}

	resp2, err := api.DescribeInstances(ctx, params2)
	if err != nil {
		return nil, err
	}

	for _, reservation := range resp2.Reservations {
		for _, instance := range reservation.Instances {
			instanceInfo := &InstanceInfo{
				InstanceID:       *instance.InstanceId,
				PrivateIP:        *instance.PrivateIpAddress,
				IPv6Address:      *instance.Ipv6Address,
				InstanceState:    instance.State.Name,
				AvailabilityZone: *instance.Placement.AvailabilityZone,
				InstanceType:     string(instance.InstanceType),
				// For Subnet lookups, we don't have ASG lifecycle state info
				LifecycleState: "",
				HealthStatus:   "",
			}

			output = append(output, instanceInfo)
		}
	}

	return output, nil
}

// Implement public interface
func (s Subnet) LookupIPs(ctx context.Context, cfg aws.Config, ipv6 bool) ([]string, error) {
	return s.doLookupIPs(ec2.NewFromConfig(cfg), ctx, ipv6)
}

// LookupInstances returns detailed information about all instances in the given subnet.
func (s Subnet) LookupInstances(ctx context.Context, cfg aws.Config) ([]*InstanceInfo, error) {
	return s.doLookupInstances(ec2.NewFromConfig(cfg), ctx)
}
