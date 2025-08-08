package lookable

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Tag is a Lookable EC2 tag name.
type Tag string

func (t Tag) String() string {
	return string(t)
}

// LookupIPs of all the instances named with the given tag.
func (t Tag) doLookupIPs(api EC2API, ctx context.Context, ipv6 bool) ([]string, error) {
	instances, err := t.doLookupInstances(api, ctx)
	if err != nil {
		return nil, err
	}

	var output []string
	for _, instance := range instances {
		output = append(output, instance.GetIP(ipv6))
	}

	return output, nil
}

// doLookupInstances returns detailed information about all instances with the given tag.
func (t Tag) doLookupInstances(api EC2API, ctx context.Context) ([]*InstanceInfo, error) {
	var output []*InstanceInfo

	params := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{t.String()},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{string(types.InstanceStateNameRunning)},
			},
		},
	}

	resp, err := api.DescribeInstances(ctx, params)
	if err != nil {
		return nil, err
	}

	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			instanceInfo := &InstanceInfo{
				InstanceID:       *instance.InstanceId,
				PrivateIP:        *instance.PrivateIpAddress,
				IPv6Address:      *instance.Ipv6Address,
				InstanceState:    instance.State.Name,
				AvailabilityZone: *instance.Placement.AvailabilityZone,
				InstanceType:     string(instance.InstanceType),
				// For Tag lookups, we don't have ASG lifecycle state info
				LifecycleState: "",
				HealthStatus:   "",
			}

			output = append(output, instanceInfo)
		}
	}

	return output, nil
}

// LookupIPs of all the instances named with the given tag.
func (t Tag) LookupIPs(ctx context.Context, cfg aws.Config, ipv6 bool) ([]string, error) {
	return t.doLookupIPs(ec2.NewFromConfig(cfg), ctx, ipv6)
}

// LookupInstances returns detailed information about all instances with the given tag.
func (t Tag) LookupInstances(ctx context.Context, cfg aws.Config) ([]*InstanceInfo, error) {
	return t.doLookupInstances(ec2.NewFromConfig(cfg), ctx)
}
