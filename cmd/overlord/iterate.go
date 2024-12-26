package main

import (
	"context"
	"html/template"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/AirVantage/overlord/pkg/changes"
	"github.com/AirVantage/overlord/pkg/lookable"
	"github.com/AirVantage/overlord/pkg/resource"
	"github.com/AirVantage/overlord/pkg/set"
	"github.com/AirVantage/overlord/pkg/state"
	"github.com/BurntSushi/toml"
	"github.com/aws/aws-sdk-go-v2/aws"
)

func iterate(ctx context.Context, cfg aws.Config, prevState *state.State) (*state.State, error) {
	var (
		resources         map[lookable.Lookable][]*resource.Resource      = make(map[lookable.Lookable][]*resource.Resource)
		resourcesToUpdate map[*resource.Resource]*changes.Changes[string] = make(map[*resource.Resource]*changes.Changes[string])
		newState          *state.State                                    = state.New()
	)

	slog.Debug("Start iteration")

	// load resources definition files
	resourcesDir, err := os.Open(filepath.Join(*configRoot, resourcesDirName))
	defer func() { resourcesDir.Close() }()
	if err != nil {
		return nil, err
	}

	resourcesFiles, err := resourcesDir.Readdir(0)
	if err != nil {
		return nil, err
	}

	for _, resourceFile := range resourcesFiles {
		if filepath.Ext(resourceFile.Name()) != ".toml" || resourceFile.IsDir() {
			continue
		}

		var rc *resource.ResourceConfig
		_, err := toml.DecodeFile(filepath.Join(*configRoot, resourcesDirName, resourceFile.Name()), &rc)
		if err != nil {
			return nil, err
		}

		slog.Debug("Reading resource file", "filename", resourceFile.Name(), "config", rc)

		rc.Resource.SrcFSInfo, err = os.Stat(filepath.Join(*configRoot, templatesDirName, rc.Resource.Src))
		if err != nil {
			return nil, err
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

	// find group ips to update
	slog.Debug("Find Resources to update")
	for g, resourcesset := range resources {

		group := g.String()
		ips, err := g.LookupIPs(ctx, cfg, *ipv6)

		// if some AWS API calls failed during the IPs lookup, stop here and exit
		// it will keep the dest file unmodified and won't execute the reload command.
		if err != nil {
			return nil, err
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
			slog.Info("Template changed", "template", file, "mod time", rc.SrcFSInfo.ModTime())
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

	// generate resources
	slog.Debug("Update resources and restart processes")
	for resource, changes := range resourcesToUpdate {
		tmpl, err := template.ParseFiles(filepath.Join(*configRoot, templatesDirName, resource.Src))
		if err != nil {
			return nil, err

		}
		err = os.MkdirAll(filepath.Dir(resource.Dest), 0777)
		if err != nil {
			return nil, err
		}
		// create the dest file and truncate it if it already exists
		destFile, err := os.Create(resource.Dest)
		defer func() { destFile.Close() }()
		if err != nil {
			return nil, err

		}
		err = tmpl.Execute(destFile, ips)
		if err != nil {
			return nil, err
		}

		slog.Info("Updating resource file from template", "resource", resource, "output", resource.Dest, "template", resource.Src)

		if resource.ReloadCmd == "" {
			continue
		}

		cmd := exec.Command("bash", "-c", resource.ReloadCmd)
		if changes != nil {
			cmd.Env = append(os.Environ(), mkEnvVar("IP_ADDED", changes.Added()), mkEnvVar("IP_REMOVED", changes.Removed()))
		}

		slog.Debug("Executing reload command", "resource", resource, "command", cmd)
		err = cmd.Start()
		if err != nil {
			return nil, err
		}
		log.Println("For resource", resource, "start reload cmd", resource.ReloadCmd)
		err = cmd.Wait()
		if err != nil {
			slog.Error("Resource reload command finished with error", "resourec", resource, "reload cmd", resource.ReloadCmd, "error", err)
		} else {
			slog.Info("Resource reload command successfull", "resourec", resource, "reload cmd", resource.ReloadCmd)
		}
	}

	slog.Debug("Iteration done", "state", newState)
	return newState, nil
}
