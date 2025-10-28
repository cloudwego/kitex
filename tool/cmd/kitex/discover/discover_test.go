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
	"testing"
)

func TestContainsTag(t *testing.T) {
	tests := []struct {
		name   string
		tags   []string
		target string
		want   bool
	}{
		{
			name:   "tag exists",
			tags:   []string{"production", "api", "v1"},
			target: "production",
			want:   true,
		},
		{
			name:   "tag exists case insensitive",
			tags:   []string{"Production", "API"},
			target: "production",
			want:   true,
		},
		{
			name:   "tag does not exist",
			tags:   []string{"staging", "api"},
			target: "production",
			want:   false,
		},
		{
			name:   "empty tags",
			tags:   []string{},
			target: "production",
			want:   false,
		},
		{
			name:   "nil tags",
			tags:   nil,
			target: "production",
			want:   false,
		},
		{
			name:   "empty target",
			tags:   []string{"production"},
			target: "",
			want:   false, // empty string should not match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsTag(tt.tags, tt.target)
			if got != tt.want {
				t.Errorf("containsTag(%v, %q) = %v, want %v", tt.tags, tt.target, got, tt.want)
			}
		})
	}
}

// Mock registry for testing
type mockRegistry struct {
	services []string
	service  map[string][]mockInstance
	listErr  error
	getErr   error
}

type mockInstance struct {
	id     string
	addr   string
	port   int
	status string
	tags   []string
}

func (m *mockRegistry) ListServices() ([]string, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.services, nil
}

func (m *mockRegistry) GetService(name string) (interface{}, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	// Return mock service data
	return m.service[name], nil
}

func (m *mockRegistry) Close() error {
	return nil
}

func TestRun_Help(t *testing.T) {
	// Test help flag
	args := []string{"--help"}
	exitCode := Run(args)
	if exitCode != 0 {
		t.Errorf("Run(--help) exit code = %d, want 0", exitCode)
	}
}

func TestRun_MissingAddr(t *testing.T) {
	// Test missing --addr flag
	args := []string{"--registry", "consul"}
	exitCode := Run(args)
	if exitCode != 1 {
		t.Errorf("Run(missing addr) exit code = %d, want 1", exitCode)
	}
}

// Note: Full integration tests with actual registry would be in a separate test file
// or require a mock registry implementation that's more sophisticated
