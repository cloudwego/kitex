/*
 * Copyright 2022 ByteDance Inc.
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
    `sync`
    `sync/atomic`
    `unsafe`

    `github.com/cloudwego/frugal/internal/rt`
)

/** Program Map **/

const (
    LoadFactor   = 0.5
    InitCapacity = 4096     // must be a power of 2
)

type ProgramMap struct {
    n uint64
    m uint32
    b []ProgramEntry
}

type ProgramEntry struct {
    vt *rt.GoType
    fn interface{}
}

func newProgramMap() *ProgramMap {
    return &ProgramMap {
        n: 0,
        m: InitCapacity - 1,
        b: make([]ProgramEntry, InitCapacity),
    }
}

func (self *ProgramMap) get(vt *rt.GoType) interface{} {
    i := self.m + 1
    p := vt.Hash & self.m

    /* linear probing */
    for ; i > 0; i-- {
        if b := self.b[p]; b.vt == vt {
            return b.fn
        } else if b.vt == nil {
            break
        } else {
            p = (p + 1) & self.m
        }
    }

    /* not found */
    return nil
}

func (self *ProgramMap) add(vt *rt.GoType, fn interface{}) *ProgramMap {
    var f float64
    var p *ProgramMap

    /* check for load factor */
    if f = float64(atomic.LoadUint64(&self.n) + 1) / float64(self.m + 1); f <= LoadFactor {
        p = self.copy()
    } else {
        p = self.rehash()
    }

    /* insert the value */
    p.insert(vt, fn)
    return p
}

func (self *ProgramMap) copy() *ProgramMap {
    p := new(ProgramMap)
    p.n = self.n
    p.m = self.m
    p.b = make([]ProgramEntry, len(self.b))
    copy(p.b, self.b)
    return p
}

func (self *ProgramMap) rehash() *ProgramMap {
    c := (self.m + 1) << 1
    r := &ProgramMap{m: c - 1, b: make([]ProgramEntry, int(c))}

    /* rehash every entry */
    for i := uint32(0); i <= self.m; i++ {
        if b := self.b[i]; b.vt != nil {
            r.insert(b.vt, b.fn)
        }
    }

    /* rebuild successful */
    return r
}

func (self *ProgramMap) insert(vt *rt.GoType, fn interface{}) {
    h := vt.Hash
    p := h & self.m

    /* linear probing */
    for i := uint32(0); i <= self.m; i++ {
        if b := &self.b[p]; b.vt != nil {
            p += 1
            p &= self.m
        } else {
            b.vt = vt
            b.fn = fn
            atomic.AddUint64(&self.n, 1)
            return
        }
    }

    /* should never happens */
    panic("no available slots")
}

/** RCU Program Cache **/

type ProgramCache struct {
    m sync.Mutex
    p unsafe.Pointer
}

func CreateProgramCache() *ProgramCache {
    return &ProgramCache {
        m: sync.Mutex{},
        p: unsafe.Pointer(newProgramMap()),
    }
}

func (self *ProgramCache) Get(vt *rt.GoType) interface{} {
    return (*ProgramMap)(atomic.LoadPointer(&self.p)).get(vt)
}

func (self *ProgramCache) Compute(vt *rt.GoType, compute func(*rt.GoType) (interface{}, error)) (interface{}, error) {
    var err error
    var val interface{}

    /* use defer to prevent inlining of this function */
    self.m.Lock()
    defer self.m.Unlock()

    /* double check with write lock held */
    if val = self.Get(vt); val != nil {
        return val, nil
    }

    /* compute the value */
    if val, err = compute(vt); err != nil {
        return nil, err
    }

    /* update the RCU cache */
    atomic.StorePointer(&self.p, unsafe.Pointer((*ProgramMap)(atomic.LoadPointer(&self.p)).add(vt, val)))
    return val, nil
}
