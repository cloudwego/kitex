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

package ttstream

import (
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
)

type Object interface {
	IsInvalid() bool
	Close() error
}

func newScavenger() *scavenger {
	s := new(scavenger)
	go s.Cleaning()
	return s
}

type scavenger struct {
	sync.RWMutex
	objects []Object
}

func (s *scavenger) Add(o Object) {
	s.Lock()
	s.objects = append(s.objects, o)
	s.Unlock()
}

func (s *scavenger) Cleaning() {
	for {
		time.Sleep(time.Second)

		s.RLock()
		for _, o := range s.objects {
			if o.IsInvalid() {
				klog.Debugf("object is invalid: %v, closing", o)
				_ = o.Close()
			}
		}
		s.RUnlock()
	}
}
