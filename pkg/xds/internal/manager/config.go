package manager

import "time"

const (
	defaultControlPlaneAddress = "istiod.istio-system.svc.cluster.local:8080"
	defaultRefreshInterval     = 5 * time.Second
	defaultXDSFetchTimeout     = time.Second
	defaultCacheExpireTime     = time.Second * 30
	defaultDumpPath            = "/tmp/xds_resource_manager.json"
)
