package lookable

import (
	"context"
	"testing"

	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestLookupSubnet(t *testing.T) {

	cases := []struct {
		client func(t *testing.T) EC2API
		subnet Subnet
		ipv6   bool
		expect []string
	}{
		/* Single instance result */
		{
			client: func(t *testing.T) EC2API {
				subnetId := "subnet-0x65432168"
				return &MockEC2API{
					DescribeSubnetsMethod: func(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
						if params.Filters == nil {
							t.Fatal("expect filters to not be nil")
						}

						return &ec2.DescribeSubnetsOutput{
							Subnets: []ec2types.Subnet{
								{
									SubnetId: &subnetId,
								},
							},
						}, nil
					},
					DescribeInstancesMethod: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
						if params.Filters == nil {
							t.Fatal("expect filters to not be nil")
						}
						if e, a := "subnet-id", subnetId; !HasEC2Filter(params.Filters, e, a) {
							t.Errorf("no filters matching %v=%v", e, a)
						}

						return &ec2.DescribeInstancesOutput{
							Reservations: []ec2types.Reservation{
								{
									Instances: []ec2types.Instance{
										{
											InstanceId:       aws.String("i-01233445"),
											PrivateIpAddress: aws.String("10.0.0.1"),
											Ipv6Address:      aws.String("f00:ba5:10:0:0:1"),
											State: &ec2types.InstanceState{
												Name: ec2types.InstanceStateNameRunning,
											},
											Placement: &ec2types.Placement{
												AvailabilityZone: aws.String("us-west-2a"),
											},
											InstanceType: ec2types.InstanceTypeT3Micro,
										},
									},
								},
							},
						}, nil
					},
				}
			},
			subnet: "mon-tag",
			ipv6:   false,

			expect: []string{"10.0.0.1"},
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctx := context.TODO()

			content, err := tt.subnet.doLookupIPs(tt.client(t), ctx, tt.ipv6)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			if !Equal(tt.expect, content) {
				t.Errorf("expect %v, got %v", tt.expect, content)
			}
		})
	}

}
