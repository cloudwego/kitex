package remoteconfig

import (
	"testing"

	"code.byted.org/kite/kitex/internal/test"
)

func TestServiceMetasGet(t *testing.T) {
	metas := new(serviceMetas)
	caller := &serviceMeta{name: "caller", tags: map[string]string{"Cluster": "callerCluster"}, category: TypeCaller}
	callee := &serviceMeta{name: "callee", tags: map[string]string{"Cluster": "calleeCluster"}, category: TypeCallee}
	existed := metas.add(caller)
	test.Assert(t, existed)
	existed = metas.add(callee)
	test.Assert(t, existed)

	existedCallers := metas.get(TypeCaller)
	test.Assert(t, len(existedCallers) == 1)
	existedCallees := metas.get(TypeCallee)
	test.Assert(t, len(existedCallees) == 1)

	deleted := metas.delete(caller, func(key string) bool { return true })
	test.Assert(t, deleted)
	deleted = metas.delete(callee, func(key string) bool { return true })
	test.Assert(t, deleted)
}

func TestNewProvider(t *testing.T) {
	p := NewProvider(nil, fetchFunc)
	test.Assert(t, p != nil)
}
