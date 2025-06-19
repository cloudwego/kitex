// Copyright 2024 CloudWeGo Authors
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
	"bytes"
	"fmt"

	"github.com/cloudwego/kitex"
	"github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/thriftgo"

	kargs "github.com/cloudwego/kitex/tool/cmd/kitex/args"
	"github.com/cloudwego/kitex/tool/cmd/kitex/code"
	"github.com/cloudwego/thriftgo/plugin"
	"github.com/cloudwego/thriftgo/sdk"
)

type KitexThriftgoPlugin struct {
	KitexParams    []string
	ThriftgoParams []string
	Pwd            string
}

func (k *KitexThriftgoPlugin) Invoke(req *plugin.Request) (res *plugin.Response) {
	return thriftgo.HandleRequest(req)
}

func (k *KitexThriftgoPlugin) GetName() string {
	return "kitex"
}

func (k *KitexThriftgoPlugin) GetPluginParameters() []string {
	return k.KitexParams
}

func (k *KitexThriftgoPlugin) GetThriftgoParameters() []string {
	return k.ThriftgoParams
}

// RunKitexThriftgoGenFromArgs kitex tool to generate thrift as sdk
func RunKitexThriftgoGenFromArgs(wd string, plugins []plugin.SDKPlugin, argsParsed *kargs.Arguments) (error, code.ToolExitCode) {
	if !argsParsed.IsThrift() {
		return fmt.Errorf("only support thrift idl"), code.ToolArgsError
	}
	p, err := GetKitexThriftgoPluginFromArgs(wd, argsParsed)
	if err != nil {
		if code.IsInterrupted(err) {
			return nil, code.ToolInterrupted
		}
		if code.IsVersionCheckFailed(err) {
			return err, code.ToolVersionCheckFailed
		}
		return err, code.ToolArgsError
	}

	plugins = append([]plugin.SDKPlugin{p}, plugins...)

	err = sdk.RunThriftgoAsSDK(wd, plugins, p.GetThriftgoParameters()...)
	if err == nil {
		return nil, code.ToolSuccess
	}
	return err, code.ToolExecuteFailed
}

// GetKitexThriftgoPlugin rawKiteXArgs is -xxx -xxx not start with kitex, for outside user
func GetKitexThriftgoPlugin(pwd string, kitexOsArgs []string) (*KitexThriftgoPlugin, error) {
	// run as kitex
	var args kargs.Arguments

	err := args.ParseArgs(kitex.Version, pwd, kitexOsArgs)
	if err != nil {
		return nil, err
	}

	return GetKitexThriftgoPluginFromArgs(pwd, &args)
}

// GetKitexThriftgoPluginFromArgs
func GetKitexThriftgoPluginFromArgs(pwd string, argsParsed *kargs.Arguments) (*KitexThriftgoPlugin, error) {
	out := new(bytes.Buffer)
	cmd, err := argsParsed.BuildCmd(out)
	if err != nil {
		return nil, err
	}

	thriftgoParams, kitexParams, err := ParseKitexThriftgoCmd(cmd)
	if err != nil {
		return nil, err
	}

	return &KitexThriftgoPlugin{
		KitexParams:    kitexParams,
		ThriftgoParams: thriftgoParams,
		Pwd:            pwd,
	}, nil
}
