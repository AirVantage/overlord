package main

import (
	"os"
	"os/exec"
	"log"
	"sort"
	"time"
	// "strings"
	"text/template"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/autoscaling"

	"github.com/BurntSushi/toml"
)

func lookupIPs(autoscalingGroup string) []string {
	as := autoscaling.New(nil)
	ec := ec2.New(nil)
	var output []string

	params1 := &autoscaling.DescribeTagsInput{
		Filters: []*autoscaling.Filter{
			{ // Required
				Name: aws.String("Value"),
				Values: []*string{
					aws.String(autoscalingGroup), 
				},
			},
		},
	}
	resp1, err := as.DescribeTags(params1)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			// Generic AWS error with Code, Message, and original error (if any)
			log.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
			if reqErr, ok := err.(awserr.RequestFailure); ok {
				// A service error occurred
				log.Println(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
			}
		} else {
			// This case should never be hit, the SDK should always return an
			// error which satisfies the awserr.Error interface.
			log.Println(err.Error())
		}
	}

	for _, tag := range resp1.Tags {
		autoscalingGroup = *tag.ResourceID
	}

	params := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{
			aws.String(autoscalingGroup), // Required
		},
	}

	resp, err := as.DescribeAutoScalingGroups(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			// Generic AWS error with Code, Message, and original error (if any)
			log.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
			if reqErr, ok := err.(awserr.RequestFailure); ok {
				// A service error occurred
				log.Println(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
			}
		} else {
			// This case should never be hit, the SDK should always return an
			// error which satisfies the awserr.Error interface.
			log.Println(err.Error())
		}

		return output
	}

	for _, instance := range resp.AutoScalingGroups[0].Instances {

		
		if *instance.LifecycleState ==  "InService" {

			params := &ec2.DescribeInstancesInput{
				InstanceIDs: []*string{
					aws.String(*instance.InstanceID), // Required
					// More values...
				},
			}


			resp, err := ec.DescribeInstances(params)
			if err != nil {
				if awsErr, ok := err.(awserr.Error); ok {
					// Generic AWS error with Code, Message, and original error (if any)
					log.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
					if reqErr, ok := err.(awserr.RequestFailure); ok {
						// A service error occurred
						log.Println(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
					}
				} else {
					// This case should never be hit, the SDK should always return an
					// error which satisfies the awserr.Error interface.
					log.Println(err.Error())
				}
				continue
			}
		
			// Pretty-print the response data.
			output = append(output, *resp.Reservations[0].Instances[0].PrivateIPAddress)
		
		}

	}

	return output

}

var (

	resourcesDirName = "/etc/overlord/resources"
	templatesDirName = "/etc/overlord/templates"
	stateFileName    = "/var/overlord/state.toml"
	interval         = 30

)

type ResourceConfig struct {
	Resource Resource `toml:"template"`
}

type Resource struct {
	Src       string
	Dest      string
	Groups    []string
	Uid       int
	Gid       int
	Mode      string
	ReloadCmd string `toml:"reload_cmd"`
}

func iterate() {

	// log.Println("Start iteration")

	resources := make(map[string]map[*Resource]bool)
	state     := make(map[string]map[string]bool)

	//load resources definition files
	resourcesDir, err := os.Open(resourcesDirName)
	defer func(){resourcesDir.Close()}()
	if err != nil { log.Fatal(err) }

	resourcesFiles, err := resourcesDir.Readdir(0)
	if err != nil { log.Fatal(err) }

	for _, resourceFile := range resourcesFiles {
		
		if filepath.Ext(resourceFile.Name()) != ".toml" ||  resourceFile.IsDir() {continue}
		
		var rc *ResourceConfig
		_, err := toml.DecodeFile(filepath.Join(resourcesDirName, resourceFile.Name()), &rc)
		if err != nil { log.Fatal(err) }

		// log.Println("Read File", resourceFile.Name(), ":", rc)

		for _, group := range rc.Resource.Groups {
			if resources[group] == nil {resources[group]=make(map[*Resource]bool)}
			resources[group][&rc.Resource] = true
		}
	}

	//load state file
	err = os.MkdirAll(filepath.Dir(stateFileName), 0777)
	if err != nil { log.Fatal(err) }

	_, err = toml.DecodeFile(stateFileName, &state)
	if err != nil && !os.IsNotExist(err) { log.Fatal(err) }	

	// log.Println("Load state from", stateFileName, ":", state)



	// log.Println("Find Resources to update")

	resourcesToUpdate := make(map[*Resource]bool)
	newState := make(map[string]map[string]bool)
	
	//find group ips to update
	for group, resourcesset := range resources {
		
		//substitute group name by var env if existing
		envSubs := os.Getenv(group)
		if envSubs != "" {
			group = envSubs
		} 

		ips := lookupIPs(group)

		newState[group] = make(map[string]bool)

		changed := false
		
		for _, ip := range ips {
			newState[group][ip] = true
			if _, stateOk := state[group][ip]; !stateOk {
				changed = true
				log.Println("For group", group, "new IP:", ip)
			}

		}

		for oldIp, _ := range state[group] {
			if _, stateOk := newState[group][oldIp]; !stateOk {
				changed = true
				log.Println("For group", group, "deprecated IP:", oldIp)
			}
		}

		if changed {
			for resource, v := range resourcesset {
				log.Println("For group", group, "update ressource:", resource)
				resourcesToUpdate[resource] = v
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
	for resource, _ := range resourcesToUpdate {
		
		tmpl, err := template.ParseFiles(filepath.Join(templatesDirName, resource.Src))
		if err != nil { log.Fatal(err) }
		err = os.MkdirAll(filepath.Dir(resource.Dest), 0777)
	    if err != nil { log.Fatal(err) }
		destFile, err := os.Create(resource.Dest)
		defer func(){destFile.Close()}()
		if err != nil { log.Fatal(err) }
		err = tmpl.Execute(destFile, ips)
		if err != nil { log.Fatal(err) }
		log.Println("For resource", resource, "update file", resource.Dest)

		if resource.ReloadCmd == "" {continue}

		//cmdSplit := strings.Fields(resource.ReloadCmd)
		//cmd := exec.Command(cmdSplit[0], cmdSplit[1:]...)
		cmd := exec.Command("bash", "-c", resource.ReloadCmd)
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
	defer func(){stateFile.Close()}()
	if err != nil { log.Fatal(err) }
	err = toml.NewEncoder(stateFile).Encode(&newState)
	state = newState
	// log.Println("Log state", state, "in file", stateFileName)

	// log.Println("Iteration done")
	
}

func main(){
	for {
		iterate()
		time.Sleep(time.Duration(interval)*time.Second)
	}
}

// func main() {
// 	for {
// 		log.Println(lookupIPs("qa-site-survey-instance"))
// 		time.Sleep(time.Duration(30)*time.Second)
// 	}
// }