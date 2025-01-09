package lookable

import (
	"context"
	"testing"

	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestTagLookupIPs(t *testing.T) {

	cases := []struct {
		client func(t *testing.T) EC2API
		tag    Tag
		ipv6   bool
		expect []string
	}{
		/* Single instance result */
		{
			client: func(t *testing.T) EC2API {
				return &MockEC2API{
					DescribeInstancesMethod: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
						if params.Filters == nil {
							t.Fatal("expect filters to not be nil")
						}

						return &ec2.DescribeInstancesOutput{
							Reservations: []ec2types.Reservation{
								{
									Instances: []ec2types.Instance{
										{
											PrivateIpAddress: aws.String("10.0.0.1"),
										},
									},
								},
							},
						}, nil
					},
				}

			},
			tag:  "mon-tag",
			ipv6: false,

			expect: []string{"10.0.0.1"},
		},
		/* Multiple instance result */
		{
			client: func(t *testing.T) EC2API {
				return &MockEC2API{
					DescribeInstancesMethod: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
						if params.Filters == nil {
							t.Fatal("expect filters to not be nil")
						}

						return &ec2.DescribeInstancesOutput{
							Reservations: []ec2types.Reservation{
								{
									Instances: []ec2types.Instance{
										{
											PrivateIpAddress: aws.String("10.0.0.1"),
										},
									},
								},
								{
									Instances: []ec2types.Instance{
										{
											PrivateIpAddress: aws.String("10.0.0.2"),
										},
									},
								},
							},
						}, nil
					},
				}

			},
			tag:  "mon-tag",
			ipv6: false,

			expect: []string{"10.0.0.1", "10.0.0.2"},
		},
		/* IPv6 results */
		{
			client: func(t *testing.T) EC2API {
				return &MockEC2API{
					DescribeInstancesMethod: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
						if params.Filters == nil {
							t.Fatal("expect filters to not be nil")
						}

						return &ec2.DescribeInstancesOutput{
							Reservations: []ec2types.Reservation{
								{
									Instances: []ec2types.Instance{
										{
											Ipv6Address: aws.String("2001:db8:51e5:5a::1"),
										},
									},
								},
							},
						}, nil
					},
				}

			},
			tag:  "mon-tag",
			ipv6: true,

			expect: []string{"2001:db8:51e5:5a::1"},
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctx := context.TODO()

			content, err := tt.tag.doLookupIPs(tt.client(t), ctx, tt.ipv6)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			if !Equal(tt.expect, content) {
				t.Errorf("expect %v, got %v", tt.expect, content)
			}
		})
	}

}

/*

 */
