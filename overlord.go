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
	"strings"
	"text/template"
	"time"

	"github.com/AirVantage/overlord/pkg/lookable"
	"github.com/AirVantage/overlord/pkg/set"

	"github.com/BurntSushi/toml"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

var (
	resourcesDirName = "/etc/overlord/resources"
	templatesDirName = "/etc/overlord/templates"
	stateFileName    = "/var/overlord/state.toml"
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

// ResourceSet is a set of unique Resources.
type ResourceSet map[*Resource]struct{}

// Add a Resource to the set.
func (rs ResourceSet) Add(r *Resource) {
	rs[r] = struct{}{}
}

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

// Formats an environment variable for one or more values.
func mkEnvVar(name string, values []string) string {
	return name + "=" + strings.Join(values, " ")
}

func iterate() {

	// log.Println("Start iteration")

	resources := make(map[lookable.Lookable]ResourceSet)

	//load resources definition files
	resourcesDir, err := os.Open(resourcesDirName)
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
		_, err := toml.DecodeFile(filepath.Join(resourcesDirName, resourceFile.Name()), &rc)
		if err != nil {
			log.Fatal(err)
		}

		// log.Println("Read File", resourceFile.Name(), ":", rc)

		for _, group := range rc.Resource.Groups {
			if resources[group] == nil {
				resources[group] = make(ResourceSet)
			}
			resources[group].Add(&rc.Resource)
		}

		for _, tag := range rc.Resource.Tags {
			if resources[tag] == nil {
				resources[tag] = make(ResourceSet)
			}
			resources[tag].Add(&rc.Resource)
		}

		for _, subnet := range rc.Resource.Subnets {
			if resources[subnet] == nil {
				resources[subnet] = make(ResourceSet)
			}
			resources[subnet].Add(&rc.Resource)
		}
	}

	state := make(map[string]set.Strings)

	//load state file
	err = os.MkdirAll(filepath.Dir(stateFileName), 0777)
	if err != nil {
		log.Fatal(err)
	}

	_, err = toml.DecodeFile(stateFileName, &state)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}

	// log.Println("Load state from", stateFileName, ":", state)

	// log.Println("Find Resources to update")

	resourcesToUpdate := make(map[*Resource]*Changes)
	newState := make(map[string]set.Strings)

	//find group ips to update
	for g, resourcesset := range resources {

		group := g.String()
		ips, err := g.LookupIPs(*ipv6)

		// if some AWS API calls failed during the IPs lookup, stop here and exit
		// it will keep the dest file unmodified and won't execute the reload command.
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				log.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
			}
			log.Fatal("AWS Error:", err.Error())
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

		if changed {
			for resource := range resourcesset {
				log.Println("For group", group, "update ressource:", resource)
				resourcesToUpdate[resource] = changes
			}
		}

	}

	//make list of ips
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

		tmpl, err := template.ParseFiles(filepath.Join(templatesDirName, resource.Src))
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

	//write state file
	stateFile, err := os.Create(stateFileName)
	defer func() { stateFile.Close() }()
	if err != nil {
		log.Fatal(err)
	}
	if err = toml.NewEncoder(stateFile).Encode(&newState); err != nil {
		log.Fatal("Error writing state file:", stateFileName, ":", err)
	}
	state = newState
	// log.Println("Log state", state, "in file", stateFileName)

	// log.Println("Iteration done")

}

func main() {
	log.SetFlags(0)
	var syslogCfg = os.Getenv("SYSLOG_ADDRESS")
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
	flag.Parse()

	// Remove the old and possibly incompatible state file.
	os.Remove(stateFileName)

	for {
		iterate()
		time.Sleep(*interval)
	}
}

// func main() {
// 	for {
// 		log.Println(lookupIPs("qa-site-survey-instance"))
// 		time.Sleep(time.Duration(30)*time.Second)
// 	}
// }
