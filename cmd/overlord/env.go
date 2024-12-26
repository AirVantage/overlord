package main

import (
	"strings"
)

// Formats an environment variable for one or more values.
func mkEnvVar(name string, values []string) string {
	return name + "=" + strings.Join(values, " ")
}
