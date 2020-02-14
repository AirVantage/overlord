package lookable

import (
	"os"
	"testing"
)

func TestLookupTag(t *testing.T) {
	tag := Tag(os.Getenv("OVERLORD_TAG"))
	if tag == "" {
		t.SkipNow()
	}
	ips, err := tag.LookupIPs(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) == 0 {
		t.Fatal("no instance found for tag:", tag)
	}
	t.Logf("%s: %+v\n", tag, ips)
}
