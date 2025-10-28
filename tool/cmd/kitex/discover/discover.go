// Copyright 2025 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package discover

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/kitex/tool/cmd/kitex/discover/formatter"
	"github.com/cloudwego/kitex/tool/cmd/kitex/discover/registry"
)

// Run executes the discover subcommand
func Run(args []string) int {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)

	var (
		registryType = fs.String("registry", "consul", "Registry type: consul, etcd, nacos, zookeeper")
		addr         = fs.String("addr", "", "Registry address (e.g., localhost:8500)")
		format       = fs.String("format", "table", "Output format: table, json, yaml")
		healthyOnly  = fs.Bool("healthy-only", false, "Show only healthy instances")
		tag          = fs.String("tag", "", "Filter by tag")
		help         = fs.Bool("help", false, "Show help message")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `kitex discover - Inspect service registries

Usage:
  kitex discover [service-name] [flags]

Examples:
  # List all services
  kitex discover --registry consul --addr localhost:8500

  # Inspect specific service
  kitex discover user-service --registry etcd --addr localhost:2379

  # JSON output for automation
  kitex discover user-service --format json

  # Only healthy instances
  kitex discover user-service --healthy-only

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help {
		fs.Usage()
		return 0
	}

	if *addr == "" {
		fmt.Fprintf(os.Stderr, "Error: --addr flag is required\n\n")
		fs.Usage()
		return 1
	}

	serviceName := ""
	if fs.NArg() > 0 {
		serviceName = fs.Arg(0)
	}

	// Create registry client
	reg, err := registry.NewRegistry(registry.Config{
		Type:    *registryType,
		Address: *addr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating registry client: %v\n", err)
		return 1
	}
	defer reg.Close()

	// Create formatter
	fmtr := formatter.New(*format)

	// Execute command
	if serviceName == "" {
		// List all services
		return listServices(reg, fmtr)
	} else {
		// Get specific service
		return getService(reg, fmtr, serviceName, *healthyOnly, *tag)
	}
}

func listServices(reg registry.Registry, fmtr formatter.Formatter) int {
	services, err := reg.ListServices()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing services: %v\n", err)
		return 1
	}

	if err := fmtr.FormatServiceList(services, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		return 1
	}

	return 0
}

func getService(reg registry.Registry, fmtr formatter.Formatter, serviceName string, healthyOnly bool, tagFilter string) int {
	service, err := reg.GetService(serviceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting service: %v\n", err)
		return 1
	}

	// Apply filters
	if healthyOnly || tagFilter != "" {
		filtered := make([]registry.ServiceInstance, 0)
		for _, inst := range service.Instances {
			if healthyOnly && inst.Status != "UP" {
				continue
			}
			if tagFilter != "" && !containsTag(inst.Tags, tagFilter) {
				continue
			}
			filtered = append(filtered, inst)
		}
		service.Instances = filtered
	}

	if err := fmtr.FormatService(service, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		return 1
	}

	return 0
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, target) {
			return true
		}
	}
	return false
}
