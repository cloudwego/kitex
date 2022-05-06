// Copyright 2021 CloudWeGo Authors
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

package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// MaxExecutionTime is a timeout for executing external plugins.
var MaxExecutionTime = time.Minute

// InsertionPointFormat is the format for insertion points.
const InsertionPointFormat = "@@thriftgo_insertion_point(%s)"

// InsertionPoint returns a new insertion point.
func InsertionPoint(names ...string) string {
	return fmt.Sprintf(InsertionPointFormat, strings.Join(names, "."))
}

// Option is used to describes an option for a plugin or a generator backend.
type Option struct {
	Name string
	Desc string
}

// Desc can be used to describes the interface of a plugin or a generator backend.
type Desc struct {
	Name    string
	Options []Option
}

// Pack packs Option into strings.
func Pack(opts []Option) (ss []string) {
	for _, o := range opts {
		ss = append(ss, o.Name+"="+o.Desc)
	}
	return
}

// ParseCompactArguments parses a compact form option into arguments.
// A compact form option is like:
//   name:key1=val1,key2,key3=val3
// This function barely checks the validity of the string, so the user should
// always provide a valid input.
func ParseCompactArguments(str string) (*Desc, error) {
	if str == "" {
		return nil, errors.New("ParseArguments: empty string")
	}
	desc := new(Desc)
	args := strings.SplitN(str, ":", 2)
	desc.Name = args[0]
	if len(args) > 1 {
		for _, a := range strings.Split(args[1], ",") {
			var opt Option
			kv := strings.SplitN(a, "=", 2)
			switch len(kv) {
			case 2:
				opt.Desc = kv[1]
				fallthrough
			case 1:
				opt.Name = kv[0]
				desc.Options = append(desc.Options, opt)
			}
		}
	}
	return desc, nil
}

// BuildErrorResponse creates a plugin response with a error message.
func BuildErrorResponse(errMsg string, warnings ...string) *Response {
	return &Response{
		Error:    &errMsg,
		Warnings: warnings,
	}
}

// Plugin takes a request and builds a response to generate codes.
type Plugin interface {
	// Name returns the name of the plugin.
	Name() string

	// Execute invokes the plugin and waits for response.
	Execute(req *Request) (res *Response)
}

// Lookup searches for PATH to find a plugin that match the description.
func Lookup(arg string) (Plugin, error) {
	parts := strings.SplitN(arg, "=", 2)

	var name, full string
	switch len(parts) {
	case 0:
		return nil, fmt.Errorf("invlaid plugin name: %s", arg)
	case 1:
		name, full = arg, "thrift-gen-"+arg
	case 2:
		name, full = parts[0], parts[1]
	}

	path, err := exec.LookPath(full)
	if err != nil {
		return nil, err
	}
	return &external{name: name, full: full, path: path}, nil
}

type external struct {
	name string
	full string
	path string
}

// Name implements the Plugin interface.
func (e *external) Name() string {
	return e.name
}

// Execute implements the Plugin interface.
func (e *external) Execute(req *Request) (res *Response) {
	data, err := Marshal(req)
	if err != nil {
		err = fmt.Errorf("failed to marshal request: %w", err)
		return BuildErrorResponse(err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), MaxExecutionTime)
	defer cancel()

	stdin := bytes.NewReader(data)
	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, e.path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, &stdout, &stderr

	if err = cmd.Run(); err != nil {
		warns := []string{
			"stdout:\n" + stdout.String(),
			"stderr:\n" + stderr.String(),
		}
		err = fmt.Errorf("execute plugin '%s' failed: %w", e.Name(), err)
		return BuildErrorResponse(err.Error(), warns...)
	}

	res, err = UnmarshalResponse(stdout.Bytes())
	if err != nil {
		err = fmt.Errorf(
			"failed to unmarshal plugin response: %w\nstdout:\n%s\nstderr:\n%s",
			err, stdout.String(), stderr.String())
		return BuildErrorResponse(err.Error())
	}
	if warn := stderr.String(); len(warn) > 0 {
		res.Warnings = append(res.Warnings, e.Name()+" stderr:\n"+warn)
	}
	return res
}
