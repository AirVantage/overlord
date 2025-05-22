package main

import (
	"strings"
	"testing"
)

// Formats an environment variable for one or more values.
func mkEnvVar(name string, values []string) string {
	return name + "=" + strings.Join(values, " ")
}

func TestMkEnvVar(t *testing.T) {
	result := mkEnvVar("IP_ADDED", []string{"192.168.1.1", "192.168.1.2"})
	expected := "IP_ADDED=192.168.1.1 192.168.1.2"
	if result != expected {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}
