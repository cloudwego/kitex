package manager

import "time"

const (
	defaultXDSFetchTimeout = time.Second
	defaultRefreshInterval = time.Second * 10
	defaultCacheExpireTime = time.Second * 10
	defaultDumpPath        = "/tmp/xds_resource_manager.json"
)
