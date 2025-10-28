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

package registry

import (
	"fmt"

	"github.com/hashicorp/consul/api"
)

// ConsulRegistry implements Registry interface for Consul
type ConsulRegistry struct {
	client *api.Client
}

// NewConsulRegistry creates a new Consul registry client
func NewConsulRegistry(address string) (Registry, error) {
	config := api.DefaultConfig()
	config.Address = address

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	return &ConsulRegistry{client: client}, nil
}

// ListServices lists all registered services in Consul
func (c *ConsulRegistry) ListServices() ([]string, error) {
	services, _, err := c.client.Catalog().Services(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}

	return names, nil
}

// GetService retrieves details of a specific service from Consul
func (c *ConsulRegistry) GetService(serviceName string) (*Service, error) {
	entries, _, err := c.client.Health().Service(serviceName, "", false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s: %w", serviceName, err)
	}

	instances := make([]ServiceInstance, 0, len(entries))
	for _, entry := range entries {
		status := "DOWN"
		if entry.Checks.AggregatedStatus() == api.HealthPassing {
			status = "UP"
		}

		instance := ServiceInstance{
			ID:       entry.Service.ID,
			Address:  entry.Service.Address,
			Port:     entry.Service.Port,
			Status:   status,
			Weight:   entry.Service.Weights.Passing,
			Metadata: entry.Service.Meta,
			Tags:     entry.Service.Tags,
		}
		instances = append(instances, instance)
	}

	return &Service{
		Name:      serviceName,
		Instances: instances,
	}, nil
}

// Close closes the Consul client connection
func (c *ConsulRegistry) Close() error {
	// Consul client doesn't require explicit cleanup
	return nil
}
