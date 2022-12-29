package main

import (
	"flag"
	"io"
	"log"
	"log/syslog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"text/template"
	"time"
	"errors"
	"context"

	"github.com/AirVantage/overlord/pkg/lookable"
	"github.com/AirVantage/overlord/pkg/set"

	"github.com/BurntSushi/toml"

	"github.com/aws/smithy-go"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

var (
	configRoot       = flag.String("etc", "/etc/overlord", "path to configuration directory")
	resourcesDirName = "resources"
	templatesDirName = "templates"
	interval         = flag.Duration("interval", 30*time.Second, "Interval between each lookup")
	ipv6             = flag.Bool("ipv6", false, "Look for IPv6 addresses instead of IPv4")
)

type ResourceConfig struct {
	Resource Resource `toml:"template"`
}

type Resource struct {
	Src       string
	Dest      string
	Groups    []lookable.AutoScalingGroup
	Tags      []lookable.Tag
	Subnets   []lookable.Subnet
	ReloadCmd string `toml:"reload_cmd"`
}

type State map[string]set.Strings

// Changes keeps track of added/removed IPs for a Resource.
// We store IPs as strings to support both IPv4 and IPv6.
type Changes struct {
	addedIPs   set.Strings
	removedIPs set.Strings
}

// NewChanges return a pointer to an initialized Changes struct.
func NewChanges() *Changes {
	return &Changes{
		addedIPs:   set.NewStringSet(),
		removedIPs: set.NewStringSet(),
	}
}

func iterate(ctx context.Context, cfg aws.Config, state State) State {
	var (
		resources map[lookable.Lookable][]*Resource = make(map[lookable.Lookable][]*Resource)
		resourcesToUpdate map[*Resource]*Changes = make(map[*Resource]*Changes)
		newState State = make(State)
	)

	// log.Println("Start iteration")

	//load resources definition files
	resourcesDir, err := os.Open(filepath.Join(*configRoot, resourcesDirName))
	defer func() { resourcesDir.Close() }()
	if err != nil {
		log.Fatal(err)
	}

	resourcesFiles, err := resourcesDir.Readdir(0)
	if err != nil {
		log.Fatal(err)
	}

	for _, resourceFile := range resourcesFiles {
		if filepath.Ext(resourceFile.Name()) != ".toml" || resourceFile.IsDir() {
			continue
		}

		var rc *ResourceConfig
		_, err := toml.DecodeFile(filepath.Join(*configRoot, resourcesDirName, resourceFile.Name()), &rc)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Read File", resourceFile.Name(), ":", rc)

		// Store each resource in a reverse map, listing resource linked to each lookable to easily match updates need per lookable changes
		for _, group := range rc.Resource.Groups {
			resources[group] = append( resources[group], &rc.Resource)
		}

		for _, tag := range rc.Resource.Tags {
			resources[tag] = append( resources[tag], &rc.Resource)
		}

		for _, subnet := range rc.Resource.Subnets {
			resources[subnet] = append( resources[subnet], &rc.Resource)
		}
	}

	// log.Println("Find Resources to update")

	//find group ips to update
	for g, resourcesset := range resources {

		group := g.String()
		ips, err := g.LookupIPs(ctx, cfg, *ipv6)

		// if some AWS API calls failed during the IPs lookup, stop here and exit
		// it will keep the dest file unmodified and won't execute the reload command.
		if err != nil {
			var oe *smithy.OperationError
			if errors.As(err, &oe) {
				log.Fatal("Failed service call processing ..: service ", oe.Service(), ", operation: ", oe.Operation(), ", error: ", oe.Unwrap())
				
			} else {
				log.Fatal("Error processing ..:", err.Error())
			}
		}

		newState[group] = set.NewStringSet()
		changes := NewChanges()
		changed := false

		if _, exists := state[group]; !exists {
			state[group] = set.NewStringSet()
		}
		for _, ip := range ips {
			newState[group].Add(ip)
			if !state[group].Has(ip) {
				changed = true
				changes.addedIPs.Add(ip)
				log.Println("For group", group, "new IP:", ip)
			}
		}

		for _, oldIP := range state[group].ToSlice() {
			if !newState[group].Has(oldIP) {
				changed = true
				changes.removedIPs.Add(oldIP)
				log.Println("For group", group, "deprecated IP:", oldIP)
			}
		}

		// handle template file change ?
		// handle resource added with existing lookable ?
		if changed {
			for _, resource := range resourcesset {
				log.Println("For group", group, "update ressource:", resource)
				resourcesToUpdate[resource] = changes
			}
		}
	}

	// Convert set to sorted array for use with text/template
	ips := make(map[string][]string)
	for group, ipsSet := range newState {
		ipsList := make([]string, 0, len(ipsSet))
		for ip := range ipsSet {
			ipsList = append(ipsList, ip)
		}
		sort.Strings(ipsList)
		ips[group] = ipsList
	}

	// log.Println("Update resources and restart processes")
	//generate resources
	for resource, changes := range resourcesToUpdate {
		tmpl, err := template.ParseFiles(filepath.Join(*configRoot, templatesDirName, resource.Src))
		if err != nil {
			log.Fatal(err)
		}
		err = os.MkdirAll(filepath.Dir(resource.Dest), 0777)
		if err != nil {
			log.Fatal(err)
		}
		// create the dest file and truncate it if it already exists
		destFile, err := os.Create(resource.Dest)
		defer func() { destFile.Close() }()
		if err != nil {
			log.Fatal(err)
		}
		err = tmpl.Execute(destFile, ips)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("For resource", resource, "update file", resource.Dest)

		if resource.ReloadCmd == "" {
			continue
		}

		//cmdSplit := strings.Fields(resource.ReloadCmd)
		//cmd := exec.Command(cmdSplit[0], cmdSplit[1:]...)
		cmd := exec.Command("bash", "-c", resource.ReloadCmd)
		if changes != nil {
			cmd.Env = append(os.Environ(), mkEnvVar("IP_ADDED", changes.addedIPs.ToSlice()), mkEnvVar("IP_REMOVED", changes.removedIPs.ToSlice()))
		}
		log.Println(cmd)
		err = cmd.Start()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("For resource", resource, "start reload cmd", resource.ReloadCmd)
		err = cmd.Wait()
		if err != nil {
			log.Println("For resource", resource, "reload cmd", resource.ReloadCmd, "finished with error", err)
		} else {
			log.Println("For resource", resource, "reload cmd", resource.ReloadCmd, "finished successfuly")
		}
	}

	// log.Println("Log state", state)
	// log.Println("Iteration done")
	return newState
}

func main() {
	var (
		syslogCfg     string
		cfg	      aws.Config
		ctx	      context.Context = context.TODO()
		runningState  State = make(State)
	)

	log.SetFlags(0)
	syslogCfg = os.Getenv("SYSLOG_ADDRESS")
	if len(syslogCfg) > 0 {
		syslogWriter, err := syslog.Dial("udp", syslogCfg, syslog.LOG_INFO, "av-balancing")
		if err != nil {
			// Do not make this error fatal. A syslog server may
			// not be available when deploying to a new region.
			log.Println("warning: cannot send logs to syslog:", err)
		} else {
			log.SetOutput(io.MultiWriter(os.Stdout, syslogWriter))
		}
	}

	// Initialise AWS SDK v2, process default configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}	

	flag.Parse()

	for {
		runningState = iterate(ctx, cfg, runningState)
		time.Sleep(*interval)
	}
}
