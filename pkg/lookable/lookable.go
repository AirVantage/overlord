package lookable

// Lookable is a group of cloud instances.
type Lookable interface {
	// LookupIPs returns the list of IP addresses of the Lookable instances, in IPv4 or IPv6.
	LookupIPs(ipv6 bool) ([]string, error)
	String() string
}
