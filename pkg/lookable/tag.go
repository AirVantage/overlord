package lookable

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Tag is a Lookable EC2 tag name.
type Tag string

func (t Tag) String() string {
	return string(t)
}

// LookupIPs of all the instances named with the given tag.
func (t Tag) LookupIPs(ipv6 bool) ([]string, error) {
	sess := session.Must(session.NewSession())
	var output []string

	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []*string{aws.String(t.String())},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []*string{aws.String(ec2.InstanceStateNameRunning)},
			},
		},
	}

	resp, err := ec2.New(sess).DescribeInstances(params)
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
