package main

import (
	"flag"
	"io"
	"log"
	"log/syslog"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"path/filepath"
	"text/template"

	"github.com/AirVantage/overlord/pkg/lookable"

	"github.com/BurntSushi/toml"
)

var (
	resourcesDirName = "/etc/overlord/resources"
	templatesDirName = "/etc/overlord/templates"
	stateFileName    = "/var/overlord/state.toml"
	interval         = 30
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
	Uid       int
	Gid       int
	Mode      string
	ReloadCmd string `toml:"reload_cmd"`
}

// Changes keeps track of added/removed IPs for a Resource.
// We use maps instead of slices to avoid duplicates.
type Changes struct {
	addedIPs   map[string]bool
	removedIPs map[string]bool
}

// NewChanges return a pointer to an initialized Changes struct.
func NewChanges() *Changes {
	return &Changes{
		addedIPs:   make(map[string]bool),
		removedIPs: make(map[string]bool),
	}
}

// Optimized way to get the keys of a map.
func keys(m map[string]bool) []string {
	result := make([]string, len(m))
	i := 0
	for key := range m {
		result[i] = key
		i++
	}
	return result
}

// Formats an environment variable for one or more values.
func mkEnvVar(name string, values map[string]bool) string {
	return name + "=" + strings.Join(keys(values), " ")
}

func iterate() {

	// log.Println("Start iteration")

	resources := make(map[lookable.Lookable]map[*Resource]bool)
	state := make(map[string]map[string]bool)

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
				resources[group] = make(map[*Resource]bool)
			}
			resources[group][&rc.Resource] = true
		}

		for _, tag := range rc.Resource.Tags {
			if resources[tag] == nil {
				resources[tag] = make(map[*Resource]bool)
			}
			resources[tag][&rc.Resource] = true
		}
	}

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
	newState := make(map[string]map[string]bool)

	//find group ips to update
	for g, resourcesset := range resources {

		//substitute group name by var env if existing

		group := g.String()
		ips, err := g.LookupIPs(*ipv6)

		// if some AWS API calls failed during the IPs lookup, stop here and exit
		// it will keep the dest file unmodified and won't execute the reload command.
		if err != nil {
			log.Fatal("AWS API call fails: ", err)
		}

		newState[group] = make(map[string]bool)
		changes := NewChanges()
		changed := false

		for _, ip := range ips {
			newState[group][ip] = true
			if _, stateOk := state[group][ip]; !stateOk {
				changed = true
				changes.addedIPs[ip] = true
				log.Println("For group", group, "new IP:", ip)
			}

		}

		for oldIp, _ := range state[group] {
			if _, stateOk := newState[group][oldIp]; !stateOk {
				changed = true
				changes.removedIPs[oldIp] = true
				log.Println("For group", group, "deprecated IP:", oldIp)
			}
		}

		if changed {
			for resource, _ := range resourcesset {
				log.Println("For group", group, "update ressource:", resource)
				resourcesToUpdate[resource] = changes
			}
		}

	}

	//make list of ips
	ips := make(map[string][]string)
	for group, ipsSet := range newState {
		ipsList := make([]string, 0, len(ipsSet))
		for ip, _ := range ipsSet {
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
			cmd.Env = append(os.Environ(), mkEnvVar("IP_ADDED", changes.addedIPs), mkEnvVar("IP_REMOVED", changes.removedIPs))
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
	err = toml.NewEncoder(stateFile).Encode(&newState)
	state = newState
	// log.Println("Log state", state, "in file", stateFileName)

	// log.Println("Iteration done")

}

func main() {
	var syslogCfg = os.Getenv("SYSLOG_ADDRESS")
	if len(syslogCfg) > 0 {
		syslogWriter, err := syslog.Dial("udp", syslogCfg, syslog.LOG_INFO, "av-balancing")
		if err != nil {
			panic(err)
		}
		log.SetOutput(io.MultiWriter(os.Stdout, syslogWriter))
	}
	flag.Parse()

	for {
		iterate()
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

// func main() {
// 	for {
// 		log.Println(lookupIPs("qa-site-survey-instance"))
// 		time.Sleep(time.Duration(30)*time.Second)
// 	}
// }
