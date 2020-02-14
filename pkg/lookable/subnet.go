package lookable

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Subnet is a Lookable AWS subnet tag name.
type Subnet string

func (s Subnet) String() string {
	return string(s)
}

// LookupIPs of all the instances belonging to the given subnet.
func (s Subnet) LookupIPs(ipv6 bool) ([]string, error) {
	sess := session.Must(session.NewSession())
	ec := ec2.New(sess)
	var output []string

	// Find the subnet
	params1 := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []*string{aws.String(s.String())},
			},
		},
	}

	resp1, err := ec.DescribeSubnets(params1)
	if err != nil {
		return nil, err
	}

	if len(resp1.Subnets) == 0 {
		return nil, fmt.Errorf("could not find subnet '%s'", s.String())
	}

	// Find the running instances
	params2 := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("subnet-id"),
				Values: []*string{resp1.Subnets[0].SubnetId},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []*string{aws.String(ec2.InstanceStateNameRunning)},
			},
		},
	}

	resp2, err := ec.DescribeInstances(params2)
	if err != nil {
		return nil, err
	}

	for _, reservation := range resp2.Reservations {
		for _, instance := range reservation.Instances {
			if ipv6 {
				output = append(output, instance.NetworkInterfaces[0].Ipv6Addresses[0].String())
			} else {
				output = append(output, *instance.PrivateIpAddress)
			}
		}
	}

	return output, nil
}
