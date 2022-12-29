package lookable
// Shared Mock interfaces for AWS SDK and compare functions for the test suite

import (
	"context"
	
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

type MockEC2API struct {
	EC2API
	DescribeSubnetsMethod   func(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeInstancesMethod func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}
func (m MockEC2API) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return m.DescribeSubnetsMethod(ctx, params, optFns...)
}
func (m MockEC2API) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return m.DescribeInstancesMethod(ctx, params, optFns...)
}


type MockASGAPI struct {
	ASGAPI
	DescribeAutoScalingGroupsMethod   func(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
}
func (m MockASGAPI) DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return m.DescribeAutoScalingGroupsMethod(ctx, params, optFns...)
}


// Equal tells whether a and b contain the same elements.
// A nil argument is equivalent to an empty slice.
func Equal[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}


// HasEC2Filter tells whether passed filter struct containt the specified key=value filter
func HasEC2Filter(filters []ec2types.Filter, key, value string) bool {
	if len(filters) == 0 {
		return false
	}
	for _,f := range filters {
		if *f.Name == key {
			for _,v := range f.Values {
				if v == value {
					return true
				}
			}
		}
	}
	return false
}

// HasASGFilter tells whether passed filter struct containt the specified key=value filter
func HasASGFilter(filters []asgtypes.Filter, key, value string) bool {
	if len(filters) == 0 {
		return false
	}
	for _,f := range filters {
		if *f.Name == key {
			for _,v := range f.Values {
				if v == value {
					return true
				}
			}
		}
	}
	return false
}
