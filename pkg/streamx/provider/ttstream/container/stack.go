/*
 * Copyright 2024 CloudWeGo Authors
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

package container

import (
	"sync"
)

func NewStack[ValueType any]() *Stack[ValueType] {
	return &Stack[ValueType]{}
}

type Stack[ValueType any] struct {
	L        sync.RWMutex
	head     *doubleLinkNode[ValueType] // head will be protected by Locker
	tail     *doubleLinkNode[ValueType] // tail will be protected by Locker
	size     int
	nodePool sync.Pool
}

func (s *Stack[ValueType]) Size() (size int) {
	s.L.RLock()
	size = s.size
	s.L.RUnlock()
	return size
}

// RangeDelete range from the stack bottom
func (s *Stack[ValueType]) RangeDelete(checking func(v ValueType) (deleteNode bool, continueRange bool)) {
	s.L.RLock()
	node := s.head
	deleteNode := false
	continueRange := true
	for node != nil && continueRange {
		deleteNode, continueRange = checking(node.val)
		if !deleteNode {
			node = node.next
			continue
		}
		// skip current node
		last := node.last
		next := node.next
		// modify last node
		if last == nil {
			// cur node is head node
			s.head = next
		} else {
			// change last next ptr
			last.next = next
		}
		// modify next node
		if next != nil {
			next.last = last
		}
		if node == s.tail {
			s.tail = last
		}
		node = node.next
		s.size -= 1
	}
	s.L.RUnlock()
}

func (s *Stack[ValueType]) Pop() (value ValueType, ok bool) {
	var node *doubleLinkNode[ValueType]
	s.L.Lock()
	if s.tail == nil {
		s.L.Unlock()
		return value, false
	}
	node = s.tail
	s.tail = node.last
	if s.tail == nil {
		s.head = nil
	} else {
		s.tail.next = nil
	}
	s.size--
	s.L.Unlock()

	value = node.val
	node.reset()
	s.nodePool.Put(node)
	return value, true
}

func (s *Stack[ValueType]) PopBottom() (value ValueType, ok bool) {
	var node *doubleLinkNode[ValueType]
	s.L.Lock()
	if s.head == nil {
		s.L.Unlock()
		return value, false
	}
	node = s.head
	s.head = s.head.next
	if s.head != nil {
		s.head.last = nil
	}
	if s.tail == node {
		s.tail = nil
	}
	s.size--
	s.L.Unlock()
	value = node.val
	node.reset()
	s.nodePool.Put(node)
	return value, true
}

func (s *Stack[ValueType]) Push(value ValueType) {
	var node *doubleLinkNode[ValueType]
	v := s.nodePool.Get()
	if v == nil {
		node = &doubleLinkNode[ValueType]{}
	} else {
		node = v.(*doubleLinkNode[ValueType])
	}
	node.val = value
	node.next = nil

	s.L.Lock()
	if s.tail == nil {
		// first node
		node.last = nil
		s.head = node
		s.tail = node
	} else {
		node.last = s.tail
		s.tail.next = node
		s.tail = s.tail.next
	}
	s.size++
	s.L.Unlock()
}
