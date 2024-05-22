package sdk

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var wd = "/Users/bytedance/code/testfield"
var IDLPath = "/Users/bytedance/code/source-code/rgo-online/rgo/tool/rgopass/rgo_gen/idl/webcast/rpc_idl/webcast/room/pack/webcast/room/webcast_room_pack.thrift"
var SimpleIDLPath = "/Users/bytedance/code/testfield/hello_base.thrift"
var params = "-module tf -thrift no_fmt" + " " + IDLPath

//func TestMulti(t *testing.T) {
//	for i := 0; i < 10; i++ {
//		TestSDK(t)
//		TestCMD(t)
//	}
//}

func TestSDK(t *testing.T) {
	startTime := time.Now()
	err := RunKitexTool(wd, strings.Split(params, " ")...)
	if err != nil {
		panic(err)
	}
	fmt.Printf("SDK Total Cost: %s\n", time.Since(startTime))
}

func TestCMD(t *testing.T) {
	startTime := time.Now()
	out, err := RunCommand(wd, "kitex", strings.Split(params, " ")...)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(out))
	fmt.Printf("CMD Total Cost: %s\n", time.Since(startTime))
}

//
//func TestSDK(t *testing.T) {
//	startTime := time.Now()
//	err := RunKitexTool(wd, strings.Split(params, " ")...)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Printf("SDK Total Cost: %s\n", time.Since(startTime))
//}
//
//func TestCMD(t *testing.T) {
//	startTime := time.Now()
//	out, err := RunCommand(wd, "kitex", strings.Split(params, " ")...)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Println(string(out))
//	fmt.Printf("CMD Total Cost: %s\n", time.Since(startTime))
//}

func RunCommand(dir, command string, args ...string) ([]byte, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("run command failed\n %s\n%s", cmd.String(), output)
		return output, err
	}
	return output, nil
}
