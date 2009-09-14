/*
 * Copyright 2022 CloudWeGo Authors
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

package utils

import (
	"sync"
	"time"
)

type PoolObject interface {
	SetDeadline(time.Time) error
	IsActive() bool // IsActive checks if the object is active.
	Expired() bool  // Expired checks if the object has expired.
	Dump() interface{}
	Close() error
}

type PoolDump struct {
	IdleNum  int           `json:"idle_num"`
	IdleList []interface{} `json:"idle_list"`
}

type PoolConfig struct {
	MinIdle        int
	MaxIdle        int
	MaxIdleTimeout time.Duration
}

type Pool struct {
	idleList []PoolObject
	mu       sync.RWMutex

	// config
	minIdle        int
	maxIdle        int           // currIdle <= maxIdle.
	maxIdleTimeout time.Duration // the idle connection will be cleaned if the idle time exceeds maxIdleTimeout.
}

func NewPool(minIdle, maxIdle int, maxIdleTimeout time.Duration) *Pool {
	p := &Pool{
		idleList:       make([]PoolObject, 0, maxIdle),
		minIdle:        minIdle,
		maxIdle:        maxIdle,
		maxIdleTimeout: maxIdleTimeout,
	}
	return p
}

// Get gets the first active objects from the idleList or construct a new object.
func (p *Pool) Get(newer func() (PoolObject, error)) (PoolObject, bool, error) {
	p.mu.Lock()
	// Get the first active one
	i := len(p.idleList) - 1
	for ; i >= 0; i-- {
		o := p.idleList[i]
		if o.IsActive() {
			p.idleList = p.idleList[:i]
			p.mu.Unlock()
			return o, true, nil
		}
		// inactive object
		o.Close()
	}
	// in case all objects are inactive
	if i < 0 {
		i = 0
	}
	p.idleList = p.idleList[:i]
	p.mu.Unlock()

	// construct a new object
	o, err := newer()
	if err != nil {
		return nil, false, err
	}
	return o, false, nil
}

func (p *Pool) Put(o PoolObject) bool {
	var recycled bool
	p.mu.Lock()
	if len(p.idleList) < p.maxIdle {
		o.SetDeadline(time.Now().Add(p.maxIdleTimeout))
		p.idleList = append(p.idleList, o)
		recycled = true
	}
	p.mu.Unlock()
	return recycled
}

// Evict clean those expired objects.
func (p *Pool) Evict() {
	p.mu.Lock()
	i := 0
	for ; i < len(p.idleList)-p.minIdle; i++ {
		if !p.idleList[i].Expired() {
			break
		}
		// close the inactive object
		p.idleList[i].Close()
	}
	p.idleList = p.idleList[i:]
	p.mu.Unlock()
}

func (p *Pool) Len() int {
	p.mu.RLock()
	l := len(p.idleList)
	p.mu.RUnlock()
	return l
}

func (p *Pool) Close() {
	p.mu.Lock()
	for i := 0; i < len(p.idleList); i++ {
		p.idleList[i].Close()
	}
	p.idleList = nil
	p.mu.Unlock()
}

func (p *Pool) Dump() PoolDump {
	p.mu.RLock()
	idleNum := len(p.idleList)
	idleList := make([]interface{}, idleNum)
	for i := 0; i < idleNum; i++ {
		idleList[i] = p.idleList[i].Dump()
	}
	s := PoolDump{
		IdleNum:  idleNum,
		IdleList: idleList,
	}
	p.mu.RUnlock()
	return s
}
