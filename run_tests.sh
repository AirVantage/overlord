#!/bin/sh

# You must declare a few environment variables before running the unit tests.

# In what AWS region are your EC2 instances located?
export AWS_REGION=

# Lookup instances by tag:Name
export OVERLORD_TAG=

# Lookup instances by AutoScalingGroup tag:Name
export OVERLORD_ASG=

# Lookup instances by Subnet tag:Name
export OVERLORD_SUBNET=

go test ./... "$@"
