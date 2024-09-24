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
	objects sync.Map
}

func (s *scavenger) Add(key string, o Object) {
	old, loaded := s.objects.LoadOrStore(key, o)
	if loaded {
		_ = old.(Object).Close()
		s.objects.Store(key, o)
	}
}

func (s *scavenger) Cleaning() {
	for {
		time.Sleep(time.Second)

		s.objects.Range(func(key, value any) bool {
			o := value.(Object)
			if !o.IsInvalid() {
				return false
			}
			klog.Debugf("object is invalid: %v, closing", o)
			_ = o.Close()
			s.objects.Delete(key)
			return true
		})
	}
}
