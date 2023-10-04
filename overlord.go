package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"text/template"
	"time"

	"github.com/AirVantage/overlord/buildvars"
	"github.com/AirVantage/overlord/pkg/changes"
	"github.com/AirVantage/overlord/pkg/lookable"
	"github.com/AirVantage/overlord/pkg/resource"
	"github.com/AirVantage/overlord/pkg/set"
	"github.com/AirVantage/overlord/pkg/state"

	"github.com/BurntSushi/toml"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go"
)

var (
	configRoot       = flag.String("etc", "/etc/overlord", "path to configuration directory")
	resourcesDirName = "resources"
	templatesDirName = "templates"
	interval         = flag.Duration("interval", 30*time.Second, "Interval between each lookup")
	ipv6             = flag.Bool("ipv6", false, "Look for IPv6 addresses instead of IPv4")
)

func iterate(ctx context.Context, cfg aws.Config, prevState *state.State) *state.State {
	var (
		resources         map[lookable.Lookable][]*resource.Resource      = make(map[lookable.Lookable][]*resource.Resource)
		resourcesToUpdate map[*resource.Resource]*changes.Changes[string] = make(map[*resource.Resource]*changes.Changes[string])
		newState          *state.State                                    = state.New()
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

		var rc *resource.ResourceConfig
		_, err := toml.DecodeFile(filepath.Join(*configRoot, resourcesDirName, resourceFile.Name()), &rc)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Read File", resourceFile.Name(), ":", rc)

		rc.Resource.SrcFSInfo, err = os.Stat(filepath.Join(*configRoot, templatesDirName, rc.Resource.Src))
		if err != nil {
			log.Fatal(err)
		}
		newState.Templates[rc.Resource.Src] = &rc.Resource

		// Store each resource in a reverse map, listing resource linked to each lookable to easily match updates need per lookable changes
		for _, group := range rc.Resource.Groups {
			resources[group] = append(resources[group], &rc.Resource)
		}

		for _, tag := range rc.Resource.Tags {
			resources[tag] = append(resources[tag], &rc.Resource)
		}

		for _, subnet := range rc.Resource.Subnets {
			resources[subnet] = append(resources[subnet], &rc.Resource)
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

		newState.Ipsets[group] = set.New[string]()
		changes := changes.New[string]()
		changed := false

		if _, exists := prevState.Ipsets[group]; !exists {
			prevState.Ipsets[group] = set.New[string]()
		}

		for _, ip := range ips {
			newState.Ipsets[group].Add(ip)
			if !prevState.Ipsets[group].Has(ip) {
				changed = true
				changes.Add(ip)
				log.Println("For group", group, "new IP:", ip)
			}
		}

		for _, oldIP := range prevState.Ipsets[group].ToSlice() {
			if !newState.Ipsets[group].Has(oldIP) {
				changed = true
				changes.Remove(oldIP)
				log.Println("For group", group, "deprecated IP:", oldIP)
			}
		}

		if changed {
			for _, resource := range resourcesset {
				log.Println("For group", group, "update ressource:", resource)

				// Merge Changes to store IP changes across differents aws resources:
				if prevChanges, exists := resourcesToUpdate[resource]; exists {
					resourcesToUpdate[resource] = prevChanges.Merge(changes)
				} else {
					resourcesToUpdate[resource] = changes
				}
			}
		}
	}

	// If new resource or template file changed since last run:
	for file, rc := range newState.Templates {
		if prevrc, exists := prevState.Templates[file]; !exists || rc.SrcFSInfo.ModTime().Sub(prevrc.SrcFSInfo.ModTime()) > 0 {
			log.Println("Template", file, "changed:", rc.SrcFSInfo.ModTime())
			if _, exists := resourcesToUpdate[rc]; !exists {
				resourcesToUpdate[rc] = changes.New[string]()
			}
		}
	}

	// Convert set to sorted array for use with text/template
	ips := make(map[string][]string)
	for group, ipsSet := range newState.Ipsets {
		ipsList := make([]string, 0, len(*ipsSet))
		for ip := range *ipsSet {
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
			cmd.Env = append(os.Environ(), mkEnvVar("IP_ADDED", changes.Added()), mkEnvVar("IP_REMOVED", changes.Removed()))
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
		syslogCfg    string
		cfg          aws.Config
		ctx          context.Context = context.TODO()
		runningState *state.State    = state.New()
	)

	fmt.Println("Version:\t",  buildvars.Version)
	fmt.Println("Build by:\t", buildvars.User)
	fmt.Println("Build at:\t", buildvars.Time)

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
