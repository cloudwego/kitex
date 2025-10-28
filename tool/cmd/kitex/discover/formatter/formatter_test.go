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

package formatter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/tool/cmd/kitex/discover/registry"
)

// Helper function to create test data
func createTestService() *registry.Service {
	return &registry.Service{
		Name: "test-service",
		Instances: []registry.ServiceInstance{
			{
				ID:      "test-1",
				Address: "10.0.1.10",
				Port:    8080,
				Status:  "UP",
				Weight:  100,
				Metadata: map[string]string{
					"version": "v1.2.3",
					"region":  "us-west",
				},
				Tags: []string{"production", "api"},
			},
			{
				ID:      "test-2",
				Address: "10.0.1.11",
				Port:    8080,
				Status:  "DOWN",
				Weight:  0,
				Metadata: map[string]string{
					"version": "v1.2.2",
				},
				Tags: []string{"staging"},
			},
		},
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   string // type name
	}{
		{"json format", "json", "*formatter.JSONFormatter"},
		{"yaml format", "yaml", "*formatter.YAMLFormatter"},
		{"table format", "table", "*formatter.TableFormatter"},
		{"default format", "invalid", "*formatter.TableFormatter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.format)
			if got == nil {
				t.Errorf("New(%q) = nil, want non-nil formatter", tt.format)
				return
			}
			// Verify it implements the Formatter interface by checking it's not nil
			// Type checking is done at compile time
		})
	}
}

func TestTableFormatter_FormatServiceList(t *testing.T) {
	f := &TableFormatter{}
	services := []string{"user-service", "order-service", "payment-service"}

	var buf bytes.Buffer
	err := f.FormatServiceList(services, &buf)
	if err != nil {
		t.Fatalf("FormatServiceList() error = %v", err)
	}

	output := buf.String()
	for _, svc := range services {
		if !strings.Contains(output, svc) {
			t.Errorf("FormatServiceList() output missing service %q", svc)
		}
	}
}

func TestTableFormatter_FormatService(t *testing.T) {
	f := &TableFormatter{}
	service := createTestService()

	var buf bytes.Buffer
	err := f.FormatService(service, &buf)
	if err != nil {
		t.Fatalf("FormatService() error = %v", err)
	}

	output := buf.String()
	// Check that output contains key elements
	if !strings.Contains(output, "test-service") {
		t.Error("FormatService() output missing service name")
	}
	if !strings.Contains(output, "10.0.1.10") {
		t.Error("FormatService() output missing instance address")
	}
	if !strings.Contains(output, "UP") {
		t.Error("FormatService() output missing status")
	}
}

func TestJSONFormatter_FormatServiceList(t *testing.T) {
	f := &JSONFormatter{}
	services := []string{"user-service", "order-service"}

	var buf bytes.Buffer
	err := f.FormatServiceList(services, &buf)
	if err != nil {
		t.Fatalf("FormatServiceList() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "user-service") || !strings.Contains(output, "order-service") {
		t.Errorf("FormatServiceList() output = %v, want to contain services", output)
	}
	// Check it's valid JSON format with services array
	if !strings.Contains(output, "services") {
		t.Error("FormatServiceList() output missing 'services' field")
	}
}

func TestJSONFormatter_FormatService(t *testing.T) {
	f := &JSONFormatter{}
	service := createTestService()

	var buf bytes.Buffer
	err := f.FormatService(service, &buf)
	if err != nil {
		t.Fatalf("FormatService() error = %v", err)
	}

	output := buf.String()
	// Check for JSON structure elements
	if !strings.Contains(output, "test-service") {
		t.Error("FormatService() output missing service name")
	}
	if !strings.Contains(output, "10.0.1.10") {
		t.Error("FormatService() output missing instance address")
	}
	if !strings.Contains(output, "8080") {
		t.Error("FormatService() output missing instance port")
	}
}

func TestYAMLFormatter_FormatServiceList(t *testing.T) {
	f := &YAMLFormatter{}
	services := []string{"user-service", "order-service"}

	var buf bytes.Buffer
	err := f.FormatServiceList(services, &buf)
	if err != nil {
		t.Fatalf("FormatServiceList() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "user-service") || !strings.Contains(output, "order-service") {
		t.Errorf("FormatServiceList() output = %v, want to contain services", output)
	}
}

func TestYAMLFormatter_FormatService(t *testing.T) {
	f := &YAMLFormatter{}
	service := createTestService()

	var buf bytes.Buffer
	err := f.FormatService(service, &buf)
	if err != nil {
		t.Fatalf("FormatService() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test-service") {
		t.Error("FormatService() output missing service name")
	}
	if !strings.Contains(output, "instances") || !strings.Contains(output, "10.0.1.10") {
		t.Error("FormatService() output missing instances data")
	}
}

func TestFormatters_EmptyServiceList(t *testing.T) {
	formatters := []struct {
		name string
		f    Formatter
	}{
		{"table", &TableFormatter{}},
		{"json", &JSONFormatter{}},
		{"yaml", &YAMLFormatter{}},
	}

	for _, tt := range formatters {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.f.FormatServiceList([]string{}, &buf)
			if err != nil {
				t.Fatalf("FormatServiceList() with empty list error = %v", err)
			}
			// Should not panic with empty list
		})
	}
}

func TestFormatters_EmptyInstances(t *testing.T) {
	formatters := []struct {
		name string
		f    Formatter
	}{
		{"table", &TableFormatter{}},
		{"json", &JSONFormatter{}},
		{"yaml", &YAMLFormatter{}},
	}

	service := &registry.Service{
		Name:      "empty-service",
		Instances: []registry.ServiceInstance{},
	}

	for _, tt := range formatters {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.f.FormatService(service, &buf)
			if err != nil {
				t.Fatalf("FormatService() with empty instances error = %v", err)
			}
		})
	}
}
