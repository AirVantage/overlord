package lookable

import (
	"os"
	"testing"
)

func TestLookupSubnet(t *testing.T) {
	subnet := Subnet(os.Getenv("OVERLORD_SUBNET"))
	if subnet == "" {
		t.SkipNow()
	}
	ips, err := subnet.LookupIPs(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) == 0 {
		t.Fatal("no instance found for subnet:", subnet)
	}
	t.Logf("%s: %+v\n", subnet, ips)
}
