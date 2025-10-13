package shmipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/cloudwego/shmipc-go"
	"golang.org/x/sync/singleflight"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
)

// Config is used to create shmIPC pool.
type Config struct {
	UnixPathBuilder func(network, address string) string
	SMConfigBuilder func(network, address string) *shmipc.SessionManagerConfig
}

// DefaultShmipcConfigWithOptions creates a shmipc configuration using provided options
func DefaultShmipcConfigWithOptions(opts *Options, psm string, shmipcAddr net.Addr) *shmipc.SessionManagerConfig {
	c := shmipc.DefaultConfig()
	c.ShareMemoryBufferCap = uint32(opts.BufferCapacity)
	if shmipc.MemMapType(opts.MemoryType) == shmipc.MemMapTypeMemFd {
		c.MemMapType = shmipc.MemMapTypeMemFd
	}
	sCfg := &shmipc.SessionManagerConfig{
		UnixPath:          shmipcAddr.String(),
		MaxStreamNum:      opts.MaxStreamNum,
		StreamMaxIdleTime: opts.StreamMaxIdleTime,
		Config:            c,
		SessionNum:        opts.Concurrency,
	}

	fmtShmPath := func(prefix string) string {
		return fmt.Sprintf("%s_%s_%d_client", prefix, psm, uint64(time.Now().UnixNano())+rand.Uint64())
	}

	if len(opts.PathPrefix) > 0 {
		sCfg.ShareMemoryPathPrefix = fmtShmPath(opts.PathPrefix)
	} else {
		sCfg.ShareMemoryPathPrefix = fmtShmPath("/dev/shm/shmipc/")
	}
	if len(opts.SliceSpec) > 0 {
		var pairs []*shmipc.SizePercentPair
		if err := json.Unmarshal([]byte(opts.SliceSpec), &pairs); err == nil {
			sCfg.BufferSliceSizes = pairs
		}
	}
	return sCfg
}

type shmipcSessionManager struct {
	instance *shmipc.SessionManager
}

func newShmipcSessionManager(smc *shmipc.SessionManagerConfig) *shmipcSessionManager {
	sm, err := shmipc.NewSessionManager(smc)
	if err != nil {
		klog.Warnf("KITEX: NewSessionManager in shm-ipc mode failed, error=%s", err.Error())
		sm = nil
	}
	return &shmipcSessionManager{instance: sm}
}

// NewShmIPCPool creates a connection pool which use shmipc stream.
// DO NOT use it directly because fallback is necessary when shm buffer is not enough,
// use NewFallbackShmIPCPool instead.
func NewShmIPCPool(config *Config) remote.LongConnPool {
	return &shmIPCPool{
		config: config,
	}
}

type shmIPCPool struct {
	config *Config
	// map[string]*shmipcSessionManager
	shmipcSessionMgrs sync.Map
	sfg               singleflight.Group
}

func (s *shmIPCPool) Get(ctx context.Context, network, address string, opt remote.ConnOption) (conn net.Conn, err error) {
	unixPath := s.config.UnixPathBuilder(network, address)
	mgr, loaded := s.shmipcSessionMgrs.Load(unixPath)
	if !loaded {
		mgr, _, _ = s.sfg.Do(unixPath, func() (interface{}, error) {
			ssm := newShmipcSessionManager(s.config.SMConfigBuilder(network, address))
			s.shmipcSessionMgrs.Store(unixPath, ssm)
			return ssm, nil
		})
	}
	ssm := mgr.(*shmipcSessionManager)
	if ssm.instance == nil {
		return nil, errors.New("cannot initialize shmipc session manager")
	}
	return ssm.instance.GetStream()
}

func (s *shmIPCPool) Put(conn net.Conn) error {
	if stream, ok := conn.(*shmipc.Stream); ok {
		mgr, ok := s.shmipcSessionMgrs.Load(stream.RemoteAddr().String())
		if ok {
			if ssm := mgr.(*shmipcSessionManager); ssm.instance != nil {
				ssm.instance.PutBack(stream)
			}
		}
		return nil
	}
	return errors.New("connection is not shmipc stream")
}

func (s *shmIPCPool) Discard(conn net.Conn) error {
	return conn.Close()
}

func (s *shmIPCPool) Close() (err error) {
	s.shmipcSessionMgrs.Range(func(key, value interface{}) bool {
		ssm := value.(*shmipcSessionManager)
		if ssm.instance != nil {
			_ = ssm.instance.Close()
		}
		return true
	})
	return
}

func (s *shmIPCPool) Clean(network, address string) {
	unixPath := s.config.UnixPathBuilder(network, address)
	s.shmipcSessionMgrs.Delete(unixPath)
}
