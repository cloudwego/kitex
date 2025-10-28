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

import "fmt"

// ServiceInstance represents a single service instance
type ServiceInstance struct {
	ID       string
	Address  string
	Port     int
	Status   string // UP, DOWN, UNKNOWN
	Weight   int
	Metadata map[string]string
	Tags     []string
}

// Service represents a service with its instances
type Service struct {
	Name      string
	Instances []ServiceInstance
}

// Registry is the interface for service discovery operations
type Registry interface {
	// ListServices lists all registered services
	ListServices() ([]string, error)

	// GetService retrieves details of a specific service
	GetService(serviceName string) (*Service, error)

	// Close closes the registry connection
	Close() error
}

// Config holds registry connection configuration
type Config struct {
	Type    string // consul, etcd, nacos, zookeeper
	Address string
}

// NewRegistry creates a new registry client based on config
func NewRegistry(cfg Config) (Registry, error) {
	switch cfg.Type {
	case "consul":
		return NewConsulRegistry(cfg.Address)
	case "etcd":
		return nil, fmt.Errorf("etcd registry not yet implemented")
	case "nacos":
		return nil, fmt.Errorf("nacos registry not yet implemented")
	case "zookeeper":
		return nil, fmt.Errorf("zookeeper registry not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported registry type: %s", cfg.Type)
	}
}
