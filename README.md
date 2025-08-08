# overlord

`overlord` is a lightweight configuration management tool focused on:

* keeping local configuration files up-to-date reagarding AWS Autoscaling Group (ASG).
* reloading applications to pick up new config file changes

It's heavily inspired by [confd](https://github.com/kelseyhightower/confd).

## Getting Started

1. Download latest release of overlord binaries: [here](https://github.com/AirVantage/overlord/releases).
2. Create a configuration file for your application in `/etc/overlord/resources/`. This [toml](https://github.com/toml-lang/toml) file (it has to have the .toml extension) describes the ASG to monitor, the configuration file to keep up-to-date and how to restart the application.
3. Create a template of your application's configuration file in `/etc/overlord/temlates/` in [golang format](http://golang.org/pkg/text/template).

Here is an example with an [HAProxy](http://www.haproxy.org/) configuration:

`/etc/overlord/resources/haproxy.toml`:

```TOML
[template]
src = "haproxy.cfg.tmpl" #template used to generate configuration file (located in /etc/overseer/temlates/)
dest = "/etc/haproxy/haproxy.cfg" #file to generate from the template
hosts = ["my-asg"] #ASG to monitor
reload_cmd = "touch /var/run/haproxy.pid; haproxy -D -f /etc/haproxy/haproxy.cfg -p /var/run/haproxy.pid -sf $(cat /var/run/haproxy.pid)" #command to reload the configuration
```

`/etc/overlord/temlates/haproxy.cf.tmpl`:

```
defaults
    mode http
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms

listen  my-app
	bind *:80
	bind *:443
	balance roundrobin
	{{range $index, $ip := index . "my-asg"}}server my-backend-{{$index}} {{$ip}}
	{{end}}
```

## Advanced Template Features

### Instance Details

In addition to IP addresses, overlord now provides detailed instance information including lifecycle state, health status, and more. This allows for more sophisticated configuration logic.

#### Template Data Structure

Templates now receive a data structure with two main sections:

- `.ips` - Backward compatible IP address lists (same as before)
- `.instances` - Detailed instance information

#### Instance Information Fields

Each instance in the `.instances` array provides:

- `InstanceID` - AWS instance ID
- `PrivateIP` - Private IPv4 address
- `IPv6Address` - IPv6 address
- `LifecycleState` - ASG lifecycle state (InService, Terminating, etc.)
- `HealthStatus` - Instance health status
- `InstanceState` - EC2 instance state (running, stopped, etc.)
- `AvailabilityZone` - AWS availability zone
- `InstanceType` - EC2 instance type
- `GetIP(ipv6 bool)` - Method to get appropriate IP address
- `IsHealthy()` - Method to check if instance is in a healthy state

#### Example Template with Instance Details

```go
# Backward compatible IP access
{{range index .ips "my-asg"}}
  IP: {{.}}
{{end}}

# New instance details access
{{range index .instances "my-asg"}}
  Instance: {{.InstanceID}}
  IP: {{.GetIP false}}
  Lifecycle State: {{.LifecycleState}}
  Health Status: {{.HealthStatus}}
  Is Healthy: {{.IsHealthy}}
{{end}}

# Conditional logic based on lifecycle state
{{range index .instances "my-asg"}}
  {{if eq .LifecycleState "InService"}}
    server backend-{{.InstanceID}} {{.GetIP false}} check
  {{else if eq .LifecycleState "Terminating"}}
    # Instance is being terminated, exclude from config
  {{end}}
{{end}}

# Only include healthy instances
{{range index .instances "my-asg"}}
  {{if .IsHealthy}}
    server healthy-backend-{{.InstanceID}} {{.GetIP false}} check
  {{end}}
{{end}}
```

This enhanced functionality allows for more sophisticated load balancer configurations, health-aware routing, and better monitoring integration.
