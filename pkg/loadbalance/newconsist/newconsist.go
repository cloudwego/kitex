package newconsist

import (
	"github.com/bytedance/gopkg/util/xxhash3"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/utils"
	"sync"
)

const (
	maxAddrLength     int     = 45 // used for construct
	nodeRatio         float64 = 2.1
	defaultNodeWeight int     = 10
)

type realNode struct {
	discovery.Instance
}

type virtualNode struct {
	realNode discovery.Instance
	value    uint64
	next     []*virtualNode
}

type ConsistInfoConfig struct {
	VirtualFactor uint32
	Weighted      bool
}

// consistent hash
type ConsistInfo struct {
	mu           sync.RWMutex
	cfg          ConsistInfoConfig
	lastRes      discovery.Result
	virtualNodes *skipList
	// cache for calculate hash
	hashByte []byte
}

func NewConsistInfo(result discovery.Result, cfg ConsistInfoConfig) *ConsistInfo {
	info := &ConsistInfo{
		cfg:          cfg,
		virtualNodes: newSkipList(),
		lastRes:      result,
	}
	info.hashByte = make([]byte, 0, utils.GetUIntLen(uint64(defaultNodeWeight*int(cfg.VirtualFactor)))+maxAddrLength+1)
	info.batchAddAllVirtual(result.Instances)
	return info
}

func (info *ConsistInfo) IsEmpty() bool {
	return len(info.lastRes.Instances) == 0
}

func (info *ConsistInfo) BuildConsistentResult(value uint64) discovery.Instance {
	info.mu.RLock()
	defer info.mu.RUnlock()

	if n := info.virtualNodes.FindGreater(value); n != nil {
		return n.realNode
	} else {
		klog.Infof("[KITEX-DEBUG] BuildConsistentResult return nil, value=%d", value)
		return nil
	}
}

func (info *ConsistInfo) Rebalance(change discovery.Change) {
	info.mu.Lock()
	defer info.mu.Unlock()

	info.lastRes = change.Result
	// update
	// TODO: optimize update logic
	if len(change.Updated) > 0 {
		info.virtualNodes = newSkipList()
		info.batchAddAllVirtual(change.Result.Instances)
		return
	}
	// add
	info.batchAddAllVirtual(change.Added)
	// delete
	for _, ins := range change.Removed {
		l := ins.Weight() * int(info.cfg.VirtualFactor)
		addrByte := utils.StringToSliceByte(ins.Address().String())
		info.removeAllVirtual(l, addrByte)
	}

}

func (info *ConsistInfo) getVirtualNodeHash(addr []byte, idx int) uint64 {
	b := info.hashByte
	b = append(b, addr...)
	b = append(b, '#')
	b = append(b, byte(idx))
	hashValue := xxhash3.Hash(b)

	b = b[:0]
	return hashValue
}

func (info *ConsistInfo) prepareByteHash(virtualNum int) {
	newCap := utils.GetUIntLen(uint64(virtualNum)) + maxAddrLength + 1
	if newCap > cap(info.hashByte) {
		info.hashByte = make([]byte, 0, newCap)
	}
}

func (info *ConsistInfo) batchAddAllVirtual(realNode []discovery.Instance) {
	totalNode := 0
	maxNodeLen := 0
	for i := 0; i < len(realNode); i++ {
		nodeLen := info.getVirtualNodeLen(realNode[i])
		if nodeLen > maxNodeLen {
			maxNodeLen = nodeLen
		}
		totalNode += nodeLen
	}
	info.prepareByteHash(maxNodeLen)

	//vns := make([]virtualNode, totalNode)

	//var idx uint64 = 0
	//estimatedTotalNode := math.Round(nodeRatio * float64(totalNode))
	//info.virtualNodes.prepareNode(int(estimatedTotalNode))
	for i := 0; i < len(realNode); i++ {
		vLen := info.getVirtualNodeLen(realNode[i])
		addrByte := utils.StringToSliceByte(realNode[i].Address().String())
		for j := 0; j < vLen; j++ {
			//vns[idx].realNode = realNode[i]
			//vns[idx].value = info.getVirtualNodeHash(addrByte, j)
			//info.virtualNodes.Insert(&vns[idx])
			//idx++
			v := &virtualNode{
				realNode: realNode[i],
				value:    info.getVirtualNodeHash(addrByte, j),
			}
			info.virtualNodes.Insert(v)
		}
	}

}

func (info *ConsistInfo) addAllVirtual(node discovery.Instance) {
	l := info.getVirtualNodeLen(node)
	addrByte := utils.StringToSliceByte(node.Address().String())

	vns := make([]virtualNode, l)
	for i := 0; i < l; i++ {
		vv := info.getVirtualNodeHash(addrByte, i)
		vns[i].realNode = node
		vns[i].value = vv
		info.virtualNodes.Insert(&vns[i])
	}
}

func (info *ConsistInfo) removeAllVirtual(virtualNum int, addrByte []byte) {
	for i := 0; i < virtualNum; i++ {
		vv := info.getVirtualNodeHash(addrByte, i)
		info.virtualNodes.Delete(vv)
	}
}

func (info *ConsistInfo) getVirtualNodeLen(node discovery.Instance) int {
	if info.cfg.Weighted {
		return node.Weight() * int(info.cfg.VirtualFactor)
	}
	return int(info.cfg.VirtualFactor)
}

// only for test
func searchRealNode(info *ConsistInfo, node *realNode) (bool, bool) {
	var (
		foundOne = false
		foundAll = true
	)
	l := info.getVirtualNodeLen(node)
	addrByte := utils.StringToSliceByte(node.Address().String())

	for i := 0; i < l; i++ {
		vv := info.getVirtualNodeHash(addrByte, i)
		ok := info.virtualNodes.Search(vv)
		if ok {
			foundOne = true
		} else {
			foundAll = false
		}
	}
	return foundOne, foundAll
}
