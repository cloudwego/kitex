package sdk

import (
	"bytes"
	"github.com/cloudwego/kitex"
	kargs "github.com/cloudwego/kitex/tool/cmd/kitex/args"
	"github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/thriftgo"
	"github.com/cloudwego/thriftgo/plugin"
	"github.com/cloudwego/thriftgo/sdk"
	"strings"
)

var args kargs.Arguments

func RunKitexTool(wd string, args ...string) error {
	plugins := []plugin.SDKPlugin{}
	return RunKitexToolWithMoreSDKPlugin(wd, plugins, args...)
}

func RunKitexToolWithMoreSDKPlugin(wd string, plugins []plugin.SDKPlugin, kitexArgs ...string) error {
	kitexPlugin, err := GetKiteXSDKPlugin(wd, kitexArgs)
	if err != nil {
		return err
	}
	s := []plugin.SDKPlugin{kitexPlugin}
	s = append(s, plugins...)

	return sdk.RunThriftgoAsSDK(wd, s, kitexPlugin.GetThriftgoParameters()...)

}

func GetKiteXSDKPlugin(pwd string, rawKiteXArgs []string) (*KiteXSDKPlugin, error) {

	// run as kitex
	err := args.ParseArgs(kitex.Version, rawKiteXArgs, pwd, false)
	if err != nil {
		return nil, err
	}

	out := new(bytes.Buffer)
	cmd, err := args.BuildCmd(out, true)
	if err != nil {
		return nil, err
	}

	// find -p kitex=xxxx and remove it.

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
	return thriftgo.RunAsSDK(req)
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
