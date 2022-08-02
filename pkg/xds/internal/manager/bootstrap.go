package manager

import (
	"fmt"
	v3core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"os"
)

const (
	POD_NAMESPACE = "POD_NAMESPACE"
	POD_NAME      = "POD_NAME"
	INSTANCE_IP   = "INSTANCE_IP"
	ISTIOD_ADDR   = "istiod.istio-system.svc:15010"
	ISTIO_VERSION = "ISTIO_VERSION"
)

type BootstrapConfig struct {
	node      *v3core.Node
	xdsSvrCfg *XDSServerConfig
}

type XDSServerConfig struct {
	serverAddress string
}

// newBootstrapConfig constructs the bootstrapConfig
func newBootstrapConfig() (*BootstrapConfig, error) {
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
	// warning log if not specified
	istioVersion := os.Getenv(ISTIO_VERSION)

	return &BootstrapConfig{
		node: &v3core.Node{
			Id: "sidecar~" + podIp + "~" + podName + "." + namespace + "~" + namespace + ".svc.cluster.local",
			Metadata: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"ISTIO_VERSION": {
						Kind: &structpb.Value_StringValue{StringValue: istioVersion},
					},
				},
			},
		},
		xdsSvrCfg: &XDSServerConfig{
			serverAddress: ISTIOD_ADDR,
		},
	}, nil
}
