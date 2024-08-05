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
	"os/exec"
	"strings"

	"github.com/cloudwego/kitex/tool/internal_pkg/log"

	"github.com/cloudwego/thriftgo/plugin"
	"github.com/cloudwego/thriftgo/sdk"

	"github.com/cloudwego/kitex"
	kargs "github.com/cloudwego/kitex/tool/cmd/kitex/args"
	"github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/thriftgo"
)

var args kargs.Arguments

var errExitZero = fmt.Errorf("os.Exit(0)")

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
				return errExitZero
			}
			return nil
		},
	})
}

func RunKitexTool(wd string, plugins []plugin.SDKPlugin, kitexArgs ...string) error {
	kitexPlugin, err := GetKiteXSDKPlugin(wd, kitexArgs)
	if err != nil {
		if err.Error() == "flag: help requested" || err == errExitZero {
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

	kitexPlugin.ThriftgoParams, kitexPlugin.KitexParams, err = grepParameters(cmd.Args)
	if err != nil {
		return nil, err
	}
	kitexPlugin.Pwd = pwd

	return kitexPlugin, nil
}

// InvokeThriftgoBySDK is for kitex tool main.go
func InvokeThriftgoBySDK(pwd string, cmd *exec.Cmd) (err error) {
	kitexPlugin := &KiteXSDKPlugin{}

	kitexPlugin.ThriftgoParams, kitexPlugin.KitexParams, err = grepParameters(cmd.Args)
	if err != nil {
		return err
	}

	kitexPlugin.Pwd = pwd

	s := []plugin.SDKPlugin{kitexPlugin}

	err = sdk.RunThriftgoAsSDK(pwd, s, kitexPlugin.GetThriftgoParameters()...)
	// when execute thriftgo as function, log will be unexpectedly replaced (for old code by design), so we have to change it back.
	log.SetDefaultLogger(log.Logger{
		Println: fmt.Fprintln,
		Printf:  fmt.Fprintf,
	})

	return err
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

func grepParameters(args []string) (thriftgoParams, kitexParams []string, err error) {
	// thriftgo -r -o kitex_gen -g go:xxx -p kitex=xxxx -p otherplugin xxx.thrift
	// ignore first argument, and remove -p kitex=xxxx

	thriftgoParams = []string{}
	kitexParams = []string{}
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("cmd args too short: %s", args)
	}

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "-p" && i+1 < len(args) {
			pluginArgs := args[i+1]
			if strings.HasPrefix(pluginArgs, "kitex") {
				kitexParams = strings.Split(pluginArgs, ",")
				i++
				continue
			}
		}
		thriftgoParams = append(thriftgoParams, arg)
	}
	return thriftgoParams, kitexParams, nil
}
