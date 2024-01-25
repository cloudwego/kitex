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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
	cfg.ThriftOptions = append(cfg.ThriftOptions, "enable_nested_struct")

	// run hessian2 options
	for _, opt := range cfg.Hessian2Options {
		runOption(cfg, opt)
	}
}

func IsHessian2(a generator.Config) bool {
	return strings.EqualFold(a.Protocol, "hessian2")
}

func EnableJavaExtension(a generator.Config) bool {
	for _, v := range a.Hessian2Options {
		if strings.EqualFold(v, JavaExtensionOption) {
			return true
		}
	}
	return false
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
	// get java.thrift, we need to put java.thrift in project root directory
	if path := util.JoinPath(cfg.OutputPath, "java.thrift"); !util.Exists(path) {
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
	// for making use of idl-ref of thriftgo, we need to check the project root directory
	idlRef := filepath.Join(cfg.OutputPath, "idl-ref.yaml")
	if !util.Exists(idlRef) {
		idlRef = filepath.Join(cfg.OutputPath, "idl-ref.yml")
	}
	file, err := os.OpenFile(idlRef, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		log.Warnf("Open %s file failed: %s\n", idlRef, err.Error())
		os.Exit(1)
	}
	defer file.Close()
	idlRefCfg := loadIDLRefConfig(file.Name(), file)

	// update the idl-ref config file
	idlRefCfg.Ref["java.thrift"] = DubboCodec + "/java"
	out, err := yaml.Marshal(idlRefCfg)
	if err != nil {
		log.Warn("Marshal configuration failed:", err.Error())
		os.Exit(1)
	}
	// clear the file content
	if err := file.Truncate(0); err != nil {
		log.Warnf("Truncate file %s failed: %s", idlRef, err.Error())
		os.Exit(1)
	}
	// set the file offset
	if _, err := file.Seek(0, 0); err != nil {
		log.Warnf("Seek file %s failed: %s", idlRef, err.Error())
		os.Exit(1)
	}
	_, err = file.Write(out)
	if err != nil {
		log.Warnf("Write to file %s failed: %s\n", idlRef, err.Error())
		os.Exit(1)
	}
}

// loadIDLRefConfig load idl-ref config from file object
func loadIDLRefConfig(fileName string, reader io.Reader) *config.RawConfig {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Warnf("Read %s file failed: %s\n", fileName, err.Error())
		os.Exit(1)
	}

	// build idl ref config
	idlRefCfg := new(config.RawConfig)
	if len(data) == 0 {
		idlRefCfg.Ref = make(map[string]interface{})
	} else {
		err := yaml.Unmarshal(data, idlRefCfg)
		if err != nil {
			log.Warnf("Parse %s file failed: %s\n", fileName, err.Error())
			log.Warn("Please check whether the idl ref configuration is correct.")
			os.Exit(2)
		}
	}

	return idlRefCfg
}

var (
	javaObjectRe                     = regexp.MustCompile(`\*java\.Object\b`)
	javaExceptionRe                  = regexp.MustCompile(`\*java\.Exception\b`)
	javaExceptionEmptyVerificationRe = regexp.MustCompile(`return p\.Exception != nil\b`)
)

// Hessian2PatchByReplace args is the arguments from command, subDirPath used for xx/xx/xx
func Hessian2PatchByReplace(args generator.Config, subDirPath string) error {
	output := args.OutputPath
	newPath := util.JoinPath(output, args.GenPath, subDirPath)
	fs, err := ioutil.ReadDir(newPath)
	if err != nil {
		return err
	}

	for _, f := range fs {
		fileName := util.JoinPath(args.OutputPath, args.GenPath, subDirPath, f.Name())
		if f.IsDir() {
			subDirPath := util.JoinPath(subDirPath, f.Name())
			if err = Hessian2PatchByReplace(args, subDirPath); err != nil {
				return err
			}
		} else if strings.HasSuffix(f.Name(), ".go") {
			data, err := ioutil.ReadFile(fileName)
			if err != nil {
				return err
			}

			data = replaceJavaObject(data)
			data = replaceJavaException(data)
			data = replaceJavaExceptionEmptyVerification(data)
			if err = ioutil.WriteFile(fileName, data, 0o644); err != nil {
				return err
			}
		}
	}

	// users do not specify -service flag, we do not need to replace
	if args.ServiceName == "" {
		return nil
	}

	handlerName := util.JoinPath(output, "handler.go")
	handler, err := ioutil.ReadFile(handlerName)
	if err != nil {
		return err
	}

	handler = replaceJavaObject(handler)
	return ioutil.WriteFile(handlerName, handler, 0o644)
}

func replaceJavaObject(content []byte) []byte {
	return javaObjectRe.ReplaceAll(content, []byte("java.Object"))
}

func replaceJavaException(content []byte) []byte {
	return javaExceptionRe.ReplaceAll(content, []byte("java.Exception"))
}

// replaceJavaExceptionEmptyVerification is used to resolve this issue:
// After generating nested struct, the generated struct would be:
//
//	 type CustomizedException struct {
//	     *java.Exception
//	 }
//
//	 It has a method:
//	 func (p *EchoCustomizedException) IsSetException() bool {
//		    return p.Exception != nil
//	 }
//
//	 After invoking replaceJavaException, *java.Exception would be converted
//	 to java.Exception and IsSetException became invalid. We convert the statement
//	 to `return true` to ignore this problem.
func replaceJavaExceptionEmptyVerification(content []byte) []byte {
	return javaExceptionEmptyVerificationRe.ReplaceAll(content, []byte("return true"))
}
