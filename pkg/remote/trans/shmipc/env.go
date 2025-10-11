package shmipc

import (
	"os"
	"strconv"

	"code.byted.org/gopkg/env"
	"github.com/cloudwego/shmipc-go"
)

const (
	// more detail: https://bytedance.feishu.cn/wiki/wikcndZtzfLltiD2eXtcBPCiC2e
	shmIPCConcurrencyKey    = "SHMIPC_CONCURRENCY"
	shmIPCBufferCapacityKey = "SHMIPC_BUFFER_CAPACITY"
	shmIPCPath              = "SHMIPC_PATH"
	shmIPCSliceSpec         = "SHMIPC_SLICE_SIZE_PERCENTS"
	shmMemoryType           = "SHMIPC_MEMORY_TYPE"
)

var global settings

type settings struct {
	ShmIPCConcurrency    int
	ShmIPCBufferCapacity int
	ShmIPCMemoryType     int
	ShmIPCPathPrefix     string
	ShmIPCSliceSpec      string
}

// ShmIPCConcurrency returns the concurrency(session num) config in shm-ipc mode
func ShmIPCConcurrency() int {
	return global.ShmIPCConcurrency
}

// ShmIPCPathPrefix return the prefix of share memory file path
func ShmIPCPathPrefix() string {
	return global.ShmIPCPathPrefix
}

// ShmIPCSliceSpec https://bytedance.feishu.cn/wiki/wikcndZtzfLltiD2eXtcBPCiC2e
func ShmIPCSliceSpec() string {
	return global.ShmIPCSliceSpec
}

// ShmIPCBufferCapacity returns the capacity of share memory buffer in shm-ipc mode
func ShmIPCBufferCapacity() int {
	return global.ShmIPCBufferCapacity
}

func ShmMemoryType() int {
	return global.ShmIPCMemoryType
}

// SetShareMemoryPath just use for test case
func SetShareMemoryPath(path string) {
	global.ShmIPCPathPrefix = path
}

func init() {
	if shmConcurrency := os.Getenv(shmIPCConcurrencyKey); shmConcurrency != "" {
		if r, e := strconv.Atoi(shmConcurrency); e == nil {
			global.ShmIPCConcurrency = r
		}
	}
	if global.ShmIPCConcurrency == 0 {
		global.ShmIPCConcurrency = 4
	}
	// default 32MB
	global.ShmIPCBufferCapacity = 32 << 20
	if capacity := os.Getenv(shmIPCBufferCapacityKey); capacity != "" {
		if c, err := strconv.Atoi(capacity); err == nil {
			global.ShmIPCBufferCapacity = c
		}
	}
	if memType := os.Getenv(shmMemoryType); memType != "" {
		if t, e := strconv.Atoi(memType); e == nil {
			global.ShmIPCMemoryType = t
		}
	} else {
		global.ShmIPCMemoryType = int(shmipc.MemMapTypeMemFd)
	}

	global.ShmIPCPathPrefix = "/dev/shm/shmipc_" + env.PSM()
	if shmPath := os.Getenv(shmIPCPath); shmPath != "" {
		global.ShmIPCPathPrefix = shmPath
	}
	if spec := os.Getenv(shmIPCSliceSpec); spec != "" {
		global.ShmIPCSliceSpec = spec
	}
}
