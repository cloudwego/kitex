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
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/cloudwego/kitex/tool/cmd/kitex/discover/registry"
)

// TableFormatter formats output as ASCII table
type TableFormatter struct{}

// FormatServiceList formats service names as a simple list
func (f *TableFormatter) FormatServiceList(services []string, w io.Writer) error {
	fmt.Fprintf(w, "Services (%d):\n", len(services))
	for _, name := range services {
		fmt.Fprintf(w, "  - %s\n", name)
	}
	return nil
}

// FormatService formats service details as a table
func (f *TableFormatter) FormatService(service *registry.Service, w io.Writer) error {
	fmt.Fprintf(w, "Service: %s\n", service.Name)
	fmt.Fprintf(w, "Instances: %d\n\n", len(service.Instances))

	if len(service.Instances) == 0 {
		fmt.Fprintln(w, "No instances found")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "INSTANCE\tSTATUS\tWEIGHT\tMETADATA\tTAGS")
	fmt.Fprintln(tw, "--------\t------\t------\t--------\t----")

	for _, inst := range service.Instances {
		address := fmt.Sprintf("%s:%d", inst.Address, inst.Port)
		if inst.Address == "" {
			address = fmt.Sprintf("(ID: %s)", inst.ID)
		}

		metadata := formatMap(inst.Metadata)
		tags := strings.Join(inst.Tags, ",")

		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n",
			address, inst.Status, inst.Weight, metadata, tags)
	}

	return tw.Flush()
}

func formatMap(m map[string]string) string {
	if len(m) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}
