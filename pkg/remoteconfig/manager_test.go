package remoteconfig

import (
	"sync/atomic"
	"testing"

	"code.byted.org/kite/kitex/internal/test"
)

var (
	fetchFunc = func(configCenter DynamicConfigCenter, callers, callees []ServiceMeta) (map[string]Config, error) {
		return map[string]Config{}, nil
	}
	p           = NewProvider(nil, fetchFunc)
	mockManager = NewConfiguratorManager(p, nil)
)

func TestConfiguratorGroupIncr(t *testing.T) {
	group := &configuratorGroup{}
	group.Incr()
	ret := atomic.LoadInt32(&group.count)
	test.Assert(t, ret == 1)

	group.Incr()
	ret = atomic.LoadInt32(&group.count)
	test.Assert(t, ret == 2)
}

func TestConfiguratorGroupDecr(t *testing.T) {
	group := &configuratorGroup{}
	atomic.AddInt32(&group.count, 2)
	group.Decr()
	ret := atomic.LoadInt32(&group.count)
	test.Assert(t, ret == 1)

	group.Decr()
	ret = atomic.LoadInt32(&group.count)
	test.Assert(t, ret == 0)
}

func TestConfiguratorGroupAdd(t *testing.T) {
	group := &configuratorGroup{}
	group.add()
	group.add()
	group.add()
	ret := atomic.LoadInt32(&group.count)
	test.Assert(t, ret == 3)
}

type mockConfig struct{}

func (m *mockConfig) Equal(am Config) bool {
	return true
}

func TestConfiguratorManagerRegisterCaller(t *testing.T) {
	g := mockManager.RegisterCaller("caller", map[string]string{"Cluster": "callerCluster"})
	test.Assert(t, g != nil)
}

func TestConfiguratorManagerRegisterCallee(t *testing.T) {
	g := mockManager.RegisterCallee("callee", map[string]string{"Cluster": "calleeCluster"})
	test.Assert(t, g != nil)
}

func TestConfigurationManager_Get(t *testing.T) {
	ret := mockManager.Get("test")
	test.Assert(t, ret == nil)
}

func TestConfigurationManager_Dump(t *testing.T) {
	ret := mockManager.Dump()
	test.Assert(t, len(ret.(map[string]interface{})) == 0)
}
