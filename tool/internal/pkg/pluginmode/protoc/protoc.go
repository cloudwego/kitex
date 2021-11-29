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

package protoc

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	gengo "google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/cloudwego/kitex/tool/internal/pkg/generator"
)

// PluginName .
const PluginName = "protoc-gen-kitex"

func Run() int {
	opts := protogen.Options{}
	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", filepath.Base(os.Args[0]), err)
		return 1
	}
	return 0
}

func run(opts protogen.Options) error {
	// unmarshal request from stdin
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(in, req); err != nil {
		return err
	}

	// init plugin
	pp := new(protocPlugin)
	pp.init()
	args := strings.Split(req.GetParameter(), ",")
	err = pp.Unpack(args)
	if err != nil {
		return fmt.Errorf("%s: unpack args: %w", PluginName, err)
	}

	// check GoPackage
	for _, f := range req.ProtoFile {
		if f == nil {
			return errors.New("ERROR: got nil ProtoFile")
		}
		if f.Options == nil || f.Options.GoPackage == nil {
			return fmt.Errorf("ERROR: go_package is missing in proto file %s", *f.Name)
		}
	}

	for _, f := range req.ProtoFile {
		if strings.Contains(*f.Options.GoPackage, generator.KitexGenPath) {
			pp.completeImportPath = true
		}
	}
	if pp.completeImportPath {
		println("WARNING: you should not use go_package with complete package path any more," +
			"use package name like thrift namespace instead of complete package path.")
	}
	// fix proto file go_package to correct dependency import
	for _, f := range req.ProtoFile {
		if !strings.Contains(*f.Options.GoPackage, generator.KitexGenPath) {
			if pp.completeImportPath {
				return errors.New("mix use of new/old proto file standard")
			}
			f.Options.GoPackage = &(&struct{ x string }{filepath.Join(pp.PackagePrefix, *f.Options.GoPackage)}).x
		}
	}

	// generate files
	gen, err := opts.New(req)
	if err != nil {
		return err
	}
	pp.process(gen)
	gen.SupportedFeatures = gengo.SupportedFeatures

	// construct plugin response
	resp := gen.Response()
	out, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(out); err != nil {
		return err
	}
	return nil
}
