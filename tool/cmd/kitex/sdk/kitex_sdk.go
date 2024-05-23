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

package sdk

import (
	"bytes"
	"flag"
	"fmt"
	"strings"

	"github.com/cloudwego/kitex"
	kargs "github.com/cloudwego/kitex/tool/cmd/kitex/args"
	"github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/thriftgo"
	"github.com/cloudwego/thriftgo/plugin"
	"github.com/cloudwego/thriftgo/sdk"
)

var args kargs.Arguments

var exitNoError = fmt.Errorf("os.Exit(0)")

func init() {
	var queryVersion bool
	args.AddExtraFlag(&kargs.ExtraFlag{
		Apply: func(f *flag.FlagSet) {
			f.BoolVar(&queryVersion, "version", false,
				"Show the version of kitex")
		},
		Check: func(a *kargs.Arguments) error {
			if queryVersion {
				println(a.Version)
				return exitNoError
			}
			return nil
		},
	})
}

func RunKitexTool(wd string, plugins []plugin.SDKPlugin, kitexArgs ...string) error {
	kitexPlugin, err := GetKiteXSDKPlugin(wd, kitexArgs)
	if err != nil {
		if err.Error() == "flag: help requested" || err == exitNoError {
			return nil
		}
		return err
	}
	s := []plugin.SDKPlugin{kitexPlugin}
	s = append(s, plugins...)

	return sdk.RunThriftgoAsSDK(wd, s, kitexPlugin.GetThriftgoParameters()...)

}

func GetKiteXSDKPlugin(pwd string, rawKiteXArgs []string) (*KiteXSDKPlugin, error) {

	// run as kitex
	err := args.ParseArgs(kitex.Version, pwd, rawKiteXArgs)
	if err != nil {
		return nil, err
	}

	out := new(bytes.Buffer)
	cmd, err := args.BuildCmd(out)
	if err != nil {
		return nil, err
	}

	kitexPlugin := &KiteXSDKPlugin{}

	kitexPluginParams := strings.Split(cmd.Args[len(cmd.Args)-2], ":")[1]
	kitexPlugin.KitexParams = strings.Split(kitexPluginParams, ",")
	kitexPlugin.ThriftgoParams = append([]string{}, cmd.Args[1:len(cmd.Args)-3]...)
	kitexPlugin.ThriftgoParams = append(kitexPlugin.ThriftgoParams, cmd.Args[len(cmd.Args)-1])
	kitexPlugin.Pwd = pwd

	return kitexPlugin, nil

}

type KiteXSDKPlugin struct {
	KitexParams    []string
	ThriftgoParams []string
	Pwd            string
}

func (k *KiteXSDKPlugin) Invoke(req *plugin.Request) (res *plugin.Response) {
	return thriftgo.HandleRequest(req)
}

func (k *KiteXSDKPlugin) GetName() string {
	return "kitex"
}

func (k *KiteXSDKPlugin) GetPluginParameters() []string {
	return k.KitexParams
}

func (k *KiteXSDKPlugin) GetThriftgoParameters() []string {
	return k.ThriftgoParams
}
