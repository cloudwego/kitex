/*
 * Copyright 2021 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package manager

import (
	"fmt"
	"os"

	v3core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	structpb "github.com/golang/protobuf/ptypes/struct"
)

const (
	POD_NAMESPACE = "POD_NAMESPACE"
	POD_NAME      = "POD_NAME"
	INSTANCE_IP   = "INSTANCE_IP"
	ISTIOD_ADDR   = "istiod.istio-system.svc:15010"
	ISTIO_VERSION = "ISTIO_VERSION"
	nodeIdSuffix  = "svc.cluster.local"
)

type BootstrapConfig struct {
	node      *v3core.Node
	xdsSvrCfg *XDSServerConfig
}

type XDSServerConfig struct {
	SvrAddr string
}

// newBootstrapConfig constructs the bootstrapConfig
func newBootstrapConfig(xdsSvrAddress string) (*BootstrapConfig, error) {
	// Get info from env
	// Info needed for construct the nodeID
	namespace := os.Getenv(POD_NAMESPACE)
	if namespace == "" {
		return nil, fmt.Errorf("[XDS] Bootstrap, POD_NAMESPACE is not set in env")
	}
	podName := os.Getenv(POD_NAME)
	if podName == "" {
		return nil, fmt.Errorf("[XDS] Bootstrap, POD_NAME is not set in env")
	}
	podIp := os.Getenv(INSTANCE_IP)
	if podIp == "" {
		return nil, fmt.Errorf("[XDS] Bootstrap, INSTANCE_IP is not set in env")
	}
	// specify the version of istio in case of the canary deployment of istiod
	istioVersion := os.Getenv(ISTIO_VERSION)

	// use default istiod address if not specified
	if xdsSvrAddress == "" {
		xdsSvrAddress = ISTIOD_ADDR
	}

	return &BootstrapConfig{
		node: &v3core.Node{
			//"sidecar~" + podIp + "~" + podName + "." + namespace + "~" + namespace + ".svc.cluster.local",
			Id: fmt.Sprintf("sidecar~%s~%s.%s~%s.%s", podIp, podName, namespace, namespace, nodeIdSuffix),
			Metadata: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"ISTIO_VERSION": {
						Kind: &structpb.Value_StringValue{StringValue: istioVersion},
					},
				},
			},
		},
		xdsSvrCfg: &XDSServerConfig{
			SvrAddr: xdsSvrAddress,
		},
	}, nil
}
