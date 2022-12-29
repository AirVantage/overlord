package lookable


import (
	"testing"
	"context"

	"strconv"
	
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

func TestLookupASG(t *testing.T) {
	
	cases := []struct {
		client func(t *testing.T) (ASGAPI,EC2API)
		asg AutoScalingGroup
		ipv6 bool
		expect []string
	}{
		/* Single instance result */
		{
			client: func(t *testing.T) (ASGAPI,EC2API) {
				instanceId := "inst-2016584701"
				
				return &MockASGAPI{
					DescribeAutoScalingGroupsMethod: func(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
						if params.Filters == nil {
							t.Fatal("expect filters to not be nil")
						}

						return &autoscaling.DescribeAutoScalingGroupsOutput{
							AutoScalingGroups: []asgtypes.AutoScalingGroup{
								{
									Instances: []asgtypes.Instance{
										{
											InstanceId: &instanceId,
										},
									},
								},
							},
							NextToken: nil,
							
						}, nil
					},
				}, &MockEC2API{
					DescribeInstancesMethod: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
						if params.Filters == nil {
							t.Fatal("expect filters to not be nil")
						}
						/*if e,a := "subnet-id", subnetId; !HasEC2Filter(params.Filters, e, a) {
							t.Errorf("no filters matching %v=%v", e, a)
						}*/
						if params.InstanceIds == nil {
							t.Fatal("expect InstancesIds to not be nil")
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
			asg: "mon-tag",
			ipv6: false,

			expect: []string{"10.0.0.1"},
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctx := context.TODO()
			asg,ec2 := tt.client(t)
			content, err := tt.asg.doLookupIPs(asg, ec2, ctx, tt.ipv6 )
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			if !Equal(tt.expect,content) {
				t.Errorf("expect %v, got %v", tt.expect, content)
			}
		})
	}
	
}
