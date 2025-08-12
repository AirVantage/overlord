package main

import (
	"context"
	"html/template"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AirVantage/overlord/pkg/changes"
	"github.com/AirVantage/overlord/pkg/lookable"
	"github.com/AirVantage/overlord/pkg/resource"
	"github.com/AirVantage/overlord/pkg/set"
	"github.com/AirVantage/overlord/pkg/state"
	"github.com/BurntSushi/toml"
	"github.com/aws/aws-sdk-go-v2/aws"
)

// loadResourceConfigs loads and parses all TOML resource configuration files
func loadResourceConfigs() (map[lookable.Lookable][]*resource.Resource, *state.State, error) {
	resources := make(map[lookable.Lookable][]*resource.Resource)
	newState := state.New()

	resourcesDir, err := os.Open(filepath.Join(*configRoot, resourcesDirName))
	if err != nil {
		return nil, nil, err
	}
	defer resourcesDir.Close()

	resourcesFiles, err := resourcesDir.Readdir(0)
	if err != nil {
		return nil, nil, err
	}

	for _, resourceFile := range resourcesFiles {
		if filepath.Ext(resourceFile.Name()) != ".toml" || resourceFile.IsDir() {
			continue
		}

		var rc *resource.ResourceConfig
		_, err := toml.DecodeFile(filepath.Join(*configRoot, resourcesDirName, resourceFile.Name()), &rc)
		if err != nil {
			return nil, nil, err
		}

		slog.Debug("Reading resource configuration",
			"filename", resourceFile.Name(),
			"configuration", rc)

		rc.Resource.SrcFSInfo, err = os.Stat(filepath.Join(*configRoot, templatesDirName, rc.Resource.Src))
		if err != nil {
			return nil, nil, err
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

	return resources, newState, nil
}

// detectChangesForGroup detects changes in instances and IPs for a specific group
func detectChangesForGroup(ctx context.Context, cfg aws.Config, g lookable.Lookable, prevState *state.State, newState *state.State) (*changes.Changes[string], error) {
	group := g.String()
	instances, err := g.LookupInstances(ctx, cfg)
	if err != nil {
		return nil, err
	}

	newState.Ipsets[group] = set.New[string]()
	newState.InstanceSets[group] = make(map[string]*lookable.InstanceInfo)
	changes := changes.New[string]()
	changed := false

	if _, exists := prevState.Ipsets[group]; !exists {
		prevState.Ipsets[group] = set.New[string]()
	}
	if _, exists := prevState.InstanceSets[group]; !exists {
		prevState.InstanceSets[group] = make(map[string]*lookable.InstanceInfo)
	}

	// Extract IPs from instances for reload script and build instance maps
	for _, instance := range instances {
		ip := instance.GetIP(*ipv6)
		newState.Ipsets[group].Add(ip)
		newState.InstanceSets[group][instance.InstanceID] = instance
	}

	// Detect changes at the instance level - this will catch state changes like "Terminating"
	// that don't necessarily change the IP address but should trigger configuration updates
	prevInstances := prevState.InstanceSets[group]
	currentInstances := newState.InstanceSets[group]

	// Check for new or changed instances
	for instanceID, currentInstance := range currentInstances {
		if prevInstance, exists := prevInstances[instanceID]; !exists {
			ip := currentInstance.GetIP(*ipv6)
			// New instance
			changed = true
			changes.Add(ip)
			slog.Info("New instance detected", "group", group, "instance", instanceID, "IP", ip)
		} else if !currentInstance.Equals(prevInstance) {
			// Instance state changed
			changed = true
			slog.Info("Instance state changed", "group", group, "instance", instanceID)
		}
	}

	// Check for removed instances
	for instanceID, prevInstance := range prevInstances {
		if _, exists := currentInstances[instanceID]; !exists {
			oldIP := prevInstance.GetIP(*ipv6)
			// Instance removed
			changed = true
			changes.Remove(oldIP)
			slog.Info("Instance removed", "group", group, "instance", instanceID, "IP", oldIP)
		}
	}

	if changed {
		return changes, nil
	}
	return nil, nil
}

// detectTemplateChanges checks if template files have changed since last run
func detectTemplateChanges(prevState *state.State, newState *state.State, resourcesToUpdate map[*resource.Resource]*changes.Changes[string]) {
	for file, rc := range newState.Templates {
		if prevrc, exists := prevState.Templates[file]; !exists || rc.SrcFSInfo.ModTime().Sub(prevrc.SrcFSInfo.ModTime()) > 0 {
			slog.Info("Template changed", "template", file, "mod time", rc.SrcFSInfo.ModTime())

			// Create an empty but non-nil change if not already existing to trigger update
			if _, exists := resourcesToUpdate[rc]; !exists {
				resourcesToUpdate[rc] = changes.New[string]()
			}
		}
	}
}

// prepareTemplateData converts state data into template-friendly format
func prepareTemplateData(newState *state.State) (map[string][]string, map[string][]*lookable.InstanceInfo) {
	ips := make(map[string][]string)
	instanceDetails := make(map[string][]*lookable.InstanceInfo)

	for group, ipsSet := range newState.Ipsets {
		ipsList := ipsSet.ToSlice()
		sort.Strings(ipsList)
		ips[group] = ipsList

		// Convert instance map to slice by sorting map keys (InstanceID) for deterministic order
		instancesMap := newState.InstanceSets[group]
		instanceIDs := make([]string, 0, len(instancesMap))
		for id := range instancesMap {
			instanceIDs = append(instanceIDs, id)
		}
		sort.Strings(instanceIDs)

		instancesSlice := make([]*lookable.InstanceInfo, 0, len(instancesMap))
		for _, id := range instanceIDs {
			instancesSlice = append(instancesSlice, instancesMap[id])
		}
		instanceDetails[group] = instancesSlice
	}

	return ips, instanceDetails
}

// generateResourceFile generates a configuration file from a template
func generateResourceFile(resource *resource.Resource, templateData map[string]interface{}) error {
	tmpl, err := template.ParseFiles(filepath.Join(*configRoot, templatesDirName, resource.Src))
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(resource.Dest), 0777)
	if err != nil {
		return err
	}

	// create the dest file and truncate it if it already exists
	destFile, err := os.Create(resource.Dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	err = tmpl.Execute(destFile, templateData)
	if err != nil {
		return err
	}

	slog.Info("Updating managed resource", "resource", resource)
	return nil
}

// executeReloadCommand executes the reload command for a resource
func executeReloadCommand(resource *resource.Resource, changes *changes.Changes[string]) error {
	if resource.ReloadCmd == "" {
		return nil
	}

	cmd := exec.Command("bash", "-c", resource.ReloadCmd)
	if changes != nil {
		cmd.Env = append(os.Environ(), mkEnvVar("IP_ADDED", changes.Added()), mkEnvVar("IP_REMOVED", changes.Removed()))
	}

	// Find and log the IP_ADDED and IP_REMOVED environment variables
	ipAdded := ""
	ipRemoved := ""
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "IP_ADDED=") {
			ipAdded = strings.TrimPrefix(env, "IP_ADDED=")
		} else if strings.HasPrefix(env, "IP_REMOVED=") {
			ipRemoved = strings.TrimPrefix(env, "IP_REMOVED=")
		}
	}

	slog.Info("Executing reload command for resource",
		"resource_template", resource.Src,
		"cmd", resource.ReloadCmd,
		"ip_added", ipAdded,
		"ip_removed", ipRemoved)

	err := cmd.Start()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		slog.Warn("Reload command failed",
			"resource_template", resource.Src,
			"cmd", resource.ReloadCmd,
			"error", err)
	} else {
		slog.Info("Reload command successful",
			"resource_template", resource.Src,
			"cmd", resource.ReloadCmd)
	}

	return nil
}

// processResourceUpdates generates files and executes reload commands for resources that need updates
func processResourceUpdates(resourcesToUpdate map[*resource.Resource]*changes.Changes[string], templateData map[string]interface{}) error {
	for resource, changes := range resourcesToUpdate {
		err := generateResourceFile(resource, templateData)
		if err != nil {
			return err
		}

		err = executeReloadCommand(resource, changes)
		if err != nil {
			return err
		}
	}
	return nil
}

func Iterate(ctx context.Context, cfg aws.Config, prevState *state.State, hupSig <-chan os.Signal) (*state.State, error) {
	slog.Debug("Start iteration")

	// Load resource configurations
	resources, newState, err := loadResourceConfigs()
	if err != nil {
		return nil, err
	}

	resourcesToUpdate := make(map[*resource.Resource]*changes.Changes[string])

	// Find resources to update based on changes
	slog.Debug("Find Resources to update")
	for g, resourcesset := range resources {
		// Check for SIGHUP signal (non-blocking)
		select {
		case <-hupSig:
			slog.Info("Received SIGHUP during iteration, forcing configuration reload")
			// Force update of all resources by marking them as changed
			for _, resource := range resourcesset {
				if _, exists := resourcesToUpdate[resource]; !exists {
					resourcesToUpdate[resource] = changes.New[string]()
				}
			}
		default:
			// No SIGHUP signal, continue normal processing
		}

		changes, err := detectChangesForGroup(ctx, cfg, g, prevState, newState)
		if err != nil {
			return nil, err
		}

		if changes != nil {
			for _, resource := range resourcesset {
				slog.Info("Instance or IP changes detected - marking resource for update",
					"group", g.String(),
					"src", resource.Src,
					"dest", resource.Dest)

				// Merge Changes to store changes across different aws resources:
				if prevChanges, exists := resourcesToUpdate[resource]; exists {
					resourcesToUpdate[resource] = prevChanges.Merge(changes)
				} else {
					resourcesToUpdate[resource] = changes
				}
			}
		}
	}

	// Check for template changes
	detectTemplateChanges(prevState, newState, resourcesToUpdate)

	// Prepare template data
	ips, instanceDetails := prepareTemplateData(newState)
	templateData := map[string]interface{}{
		"ips":       ips,
		"instances": instanceDetails,
	}

	// Generate resources and restart processes
	slog.Debug("Update resources and restart processes")
	err = processResourceUpdates(resourcesToUpdate, templateData)
	if err != nil {
		return nil, err
	}

	slog.Debug("Iteration done", "state", newState)
	return newState, nil
}
