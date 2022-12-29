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

	var output []string

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
			if ipv6 {
				output = append(output, *instance.NetworkInterfaces[0].Ipv6Addresses[0].Ipv6Address)
			} else {
				output = append(output, *instance.PrivateIpAddress)
			}
		}
	}

	return output, nil
}

// LookupIPs of all the instances named with the given tag.
func (t Tag) LookupIPs(ctx context.Context, cfg aws.Config, ipv6 bool) ([]string, error) {
	return t.doLookupIPs( ec2.NewFromConfig(cfg), ctx, ipv6)
}
