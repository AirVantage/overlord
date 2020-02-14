package lookable

import (
	"os"
	"testing"
)

func TestLookupASG(t *testing.T) {
	asg := AutoScalingGroup(os.Getenv("OVERLORD_ASG"))
	if asg == "" {
		t.SkipNow()
	}
	ips, err := asg.LookupIPs(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) == 0 {
		t.Fatal("no instance found for asg:", asg)
	}
	t.Logf("%s: %+v\n", asg, ips)
}
