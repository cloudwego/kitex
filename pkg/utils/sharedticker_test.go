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
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	mockutils "github.com/cloudwego/kitex/internal/mocks/utils"
	"github.com/cloudwego/kitex/internal/test"
)

func TestSharedTicker_Add(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rt := mockutils.NewMockRefreshTask(ctrl)
	rt.EXPECT().Refresh().AnyTimes()
	st := NewSharedTicker(1)
	st.Add(nil)
	test.Assert(t, len(st.tasks) == 0)
	st.Add(rt)
	test.Assert(t, len(st.tasks) == 1)
	test.Assert(t, !st.Closed())
}

func TestSharedTicker_DeleteAndClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := NewSharedTicker(1)
	var (
		num   = 10
		tasks = make([]RefreshTask, num)
	)
	for i := 0; i < num; i++ {
		rt := mockutils.NewMockRefreshTask(ctrl)
		rt.EXPECT().Refresh().AnyTimes()
		tasks[i] = rt
		st.Add(rt)
	}
	test.Assert(t, len(st.tasks) == num)
	for i := 0; i < num; i++ {
		st.Delete(tasks[i])
	}
	test.Assert(t, len(st.tasks) == 0)
	test.Assert(t, st.Closed())
}

func TestSharedTicker_Tick(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	duration := 100 * time.Millisecond
	st := NewSharedTicker(duration)
	var (
		num   = 10
		tasks = make([]RefreshTask, num)
	)
	for i := 0; i < num; i++ {
		rt := mockutils.NewMockRefreshTask(ctrl)
		rt.EXPECT().Refresh().Times(1) // all tasks should be refreshed once during the test
		tasks[i] = rt
		st.Add(rt)
	}
	time.Sleep(150 * time.Millisecond)
}
