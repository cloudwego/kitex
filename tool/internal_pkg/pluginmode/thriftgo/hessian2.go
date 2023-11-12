// Copyright 2023 CloudWeGo Authors
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

package thriftgo

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cloudwego/thriftgo/config"
	"gopkg.in/yaml.v3"

	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
)

const (
	JavaExtensionOption = "java_extension"

	DubboCodec = "github.com/kitex-contrib/codec-dubbo"
	JavaThrift = "https://raw.githubusercontent.com/kitex-contrib/codec-dubbo/main/java/java.thrift"
)

// Hessian2PreHook Hook before building cmd
func Hessian2PreHook(cfg *generator.Config) {
	// add thrift option
	cfg.ThriftOptions = append(cfg.ThriftOptions, "template=slim,with_reflection,code_ref")

	// run hessian2 options
	for _, opt := range cfg.Hessian2Options {
		runOption(cfg, opt)
	}
}

// runOption Execute the corresponding function according to the option
func runOption(cfg *generator.Config, opt string) {
	switch opt {
	case JavaExtensionOption:
		runJavaExtensionOption(cfg)
	}
}

// runJavaExtensionOption Pull the extension file of java class from remote
func runJavaExtensionOption(cfg *generator.Config) {
	// get java.thrift
	if path := util.JoinPath(filepath.Dir(cfg.IDL), "java.thrift"); !util.Exists(path) {
		if err := util.DownloadFile(JavaThrift, path); err != nil {
			log.Warn("Downloading java.thrift file failed:", err.Error())
			abs, err := filepath.Abs(path)
			if err != nil {
				abs = path
			}
			log.Warnf("You can try to download again. If the download still fails, you can choose to manually download \"%s\" to the local path \"%s\".", JavaThrift, abs)
			os.Exit(1)
		}
	}

	// merge idl-ref configuration patch
	patchIDLRefConfig(cfg)
}

// patchIDLRefConfig merge idl-ref configuration patch
func patchIDLRefConfig(cfg *generator.Config) {
	// load the idl-ref config file (create it first if it does not exist)
	idlRef := filepath.Join(filepath.Dir(cfg.IDL), "idl-ref.yaml")
	if !util.Exists(idlRef) {
		idlRef = filepath.Join(filepath.Dir(cfg.IDL), "idl-ref.yml")
	}
	file, err := os.OpenFile(idlRef, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0o644)
	if err != nil {
		log.Warnf("Open %s file failed: %s\n", idlRef, err.Error())
		os.Exit(1)
	}
	defer file.Close()
	idlRefCfg := loadIDLRefConfig(file)

	// update the idl-ref config file
	idlRefCfg.Ref["java.thrift"] = DubboCodec + "/java"
	out, err := yaml.Marshal(idlRefCfg)
	if err != nil {
		log.Warn("Marshal configuration failed:", err.Error())
		os.Exit(1)
	}
	_, err = file.Write(out)
	if err != nil {
		log.Warnf("Write to file %s failed: %s\n", idlRef, err.Error())
		os.Exit(1)
	}
}

// loadIDLRefConfig load idl-ref config from file object
func loadIDLRefConfig(file *os.File) *config.RawConfig {
	data, err := ioutil.ReadAll(file)
	if err != nil {
		log.Warnf("Read %s file failed: %s\n", file.Name(), err.Error())
		os.Exit(1)
	}

	// build idl ref config
	idlRefCfg := new(config.RawConfig)
	if len(data) == 0 {
		idlRefCfg.Ref = make(map[string]interface{})
	} else {
		err := yaml.Unmarshal(data, idlRefCfg)
		if err != nil {
			log.Warnf("Parse %s file failed: %s\n", file.Name(), err.Error())
			log.Warn("Please check whether the idl ref configuration is correct.")
			os.Exit(2)
		}
	}

	return idlRefCfg
}
