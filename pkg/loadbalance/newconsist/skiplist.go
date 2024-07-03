package newconsist

import "github.com/bytedance/gopkg/lang/fastrand"

const (
	MAX_LEVEL = 32 // max level of skip list
)

// TODO: optimize allocation
// skipList for consistent hash
// not concurrent safe
type skipList struct {
	dummy      *virtualNode
	totalLevel int

	// only for insert and delete
	updateCache []*virtualNode
	// node cache
	nodeCache []*virtualNode
}

// newSkipList returns a new skip list
func newSkipList() *skipList {
	return &skipList{
		dummy:       &virtualNode{nil, 0, make([]*virtualNode, MAX_LEVEL)},
		totalLevel:  1,
		updateCache: make([]*virtualNode, MAX_LEVEL),
	}
}

// Insert inserts a node into the skip list
func (sl *skipList) Insert(n *virtualNode) {
	level := sl.randomLevel()
	// temporary slice for update
	var update []*virtualNode
	if sl.updateCache != nil {
		update = sl.updateCache[:maxValue(level, sl.totalLevel)]
	} else {
		update = make([]*virtualNode, maxValue(level, sl.totalLevel))
	}

	if level > sl.totalLevel {
		// grow new level
		for i := sl.totalLevel; i < level; i++ {
			update[i-1] = sl.dummy
		}
		sl.totalLevel = level
	}

	// search the node with greater value than the new value on each level
	current := sl.dummy
	for i := sl.totalLevel - 1; i >= 0; i-- {
		for len(current.next) > i && current.next[i] != nil && current.next[i].value < n.value {
			current = current.next[i]
		}
		update[i] = current
	}

	// insert the node into the [0:level] levels of the list
	newNode := n
	n.next = sl.makeNewVirtualNode(level)
	for i := 0; i < level; i++ {
		newNode.next[i] = update[i].next[i]
		update[i].next[i] = newNode
	}
}

// Delete removes the nodes with the input value
func (sl *skipList) Delete(value uint64) {
	// temporary slice for update
	var update []*virtualNode
	if sl.updateCache != nil {
		update = sl.updateCache[:sl.totalLevel]
	} else {
		update = make([]*virtualNode, sl.totalLevel)
	}

	current := sl.dummy
	// search the node with equal or greater value than the new value on each level
	for i := sl.totalLevel - 1; i >= 0; i-- {
		for current.next[i] != nil && current.next[i].value < value {
			current = current.next[i]
		}
		update[i] = current
	}

	current = current.next[0]
	if current != nil && current.value == value {
		// if the value is found, remove the node
		for i := 0; i < sl.totalLevel; i++ {
			// check from low to high level
			// if exist in the current level, remove it. Otherwise, break the loop
			if update[i].next[i] != current {
				break
			}
			update[i].next[i] = current.next[i]
		}

		// check from high to low level
		// if no node in one level, remove this level
		for sl.totalLevel > 1 && sl.dummy.next[sl.totalLevel-1] == nil {
			sl.totalLevel--
		}
	}
}

// Search checks if the value can be found in the skip list
func (sl *skipList) Search(value uint64) bool {
	current := sl.dummy
	for i := sl.totalLevel - 1; i >= 0; i-- {
		for current.next[i] != nil && current.next[i].value < value {
			current = current.next[i]
		}
	}
	current = current.next[0]
	return current != nil && current.value == value
}

// FindGreater finds the first node with greater value than the input value
func (sl *skipList) FindGreater(value uint64) *virtualNode {
	current := sl.dummy
	for i := sl.totalLevel - 1; i >= 0; i-- {
		for current.next[i] != nil && current.next[i].value <= value {
			current = current.next[i]
		}
	}
	if res := current.next[0]; res != nil {
		return res
	} else {
		// return the first node if not found since the skip list is treated as a ring
		return sl.dummy.next[0]
	}
}

// randomLevel returns a random level for a new node
func (sl *skipList) randomLevel() int {
	level := 1
	for fastrand.Float32() < 0.5 && level < MAX_LEVEL {
		level++
	}
	return level
}

func (sl *skipList) prepareNode(num int) {
	if len(sl.nodeCache) < num {
		sl.nodeCache = append(sl.nodeCache, make([]*virtualNode, num)...)
	}
}

func (sl *skipList) makeNewVirtualNode(num int) []*virtualNode {
	cacheLen := len(sl.nodeCache)
	var res []*virtualNode
	if cacheLen > num {
		res = sl.nodeCache[:num]
		sl.nodeCache = sl.nodeCache[num:]
	} else {
		res = make([]*virtualNode, num)
	}
	return res
}

func maxValue(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}
