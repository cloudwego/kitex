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

package profiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime/pprof"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/pprof/profile"

	"github.com/cloudwego/kitex/pkg/klog"
)

type profilerContextKey struct{}

type Profiler interface {
	Run(ctx context.Context) (err error)
	Stop()
	Prepare(ctx context.Context) context.Context
	Tag(ctx context.Context, tags ...string) context.Context
	Untag(ctx context.Context)
	Lookup(ctx context.Context, key string) (string, bool)
}

type Processor func(profiles []*TagsProfile) error

func LogProcessor(profiles []*TagsProfile) error {
	if len(profiles) == 0 {
		return nil
	}
	klog.Infof("KITEX: profiler collect %d records", len(profiles))
	for _, p := range profiles {
		if p.Key != "" {
			klog.Infof("KITEX: profiler - %s %.2f%%", p.Key, p.Percent*100)
		} else {
			klog.Infof("KITEX: profiler - type=default %.2f%%", p.Percent*100)
		}
	}
	klog.Info("---------------------------------")
	return nil
}

func NewProfiler(processor Processor, interval, window time.Duration) *profiler {
	if processor == nil {
		processor = LogProcessor
	}
	return &profiler{
		processor: processor,
		interval:  interval,
		window:    window,
	}
}

var _ Profiler = (*profiler)(nil)

type profiler struct {
	data    bytes.Buffer // protobuf
	stopped int32
	// settings
	processor Processor
	interval  time.Duration
	window    time.Duration
}

type profilerContext struct {
	untagCtx context.Context
	tags     []string
}

func newProfilerContext() *profilerContext {
	return &profilerContext{
		tags: make([]string, 0, 12),
	}
}

func (p *profiler) Prepare(ctx context.Context) context.Context {
	if c := ctx.Value(profilerContextKey{}); c != nil {
		return ctx
	}
	return context.WithValue(ctx, profilerContextKey{}, newProfilerContext())
}

func (p *profiler) Stop() {
	atomic.StoreInt32(&p.stopped, 1)
}

func (p *profiler) Run(ctx context.Context) (err error) {
	defer pprof.SetGoroutineLabels(ctx)
	pprof.SetGoroutineLabels(ctx)

	var profiles []*TagsProfile
	for {
		if atomic.LoadInt32(&p.stopped) != 0 {
			return nil
		}

		// wait for an internal time to reduce the cost of profiling
		if p.interval > 0 {
			time.Sleep(p.interval)
		}

		// start profiler
		if err = p.startProfile(); err != nil {
			klog.Errorf("KITEX: profiler start profile failed: %v", err)
			// the reason of startProfile failed maybe because of there is a running pprof program.
			// so we need to wait a fixed time for it.
			time.Sleep(time.Second * 30)
			continue
		}
		// wait for a window time to collect pprof data
		time.Sleep(p.window)
		// stop profiler
		p.stopProfile()

		// analyse and process pprof data
		profiles, err = p.analyse()
		if err != nil {
			klog.Errorf("KITEX: profiler analyse failed: %v", err)
			continue
		}
		err = p.processor(profiles)
		if err != nil {
			klog.Errorf("KITEX: profiler controller process failed: %v", err)
			continue
		}
	}
}

func (p *profiler) Tag(ctx context.Context, tags ...string) context.Context {
	pctx, ok := ctx.Value(profilerContextKey{}).(*profilerContext)
	if !ok {
		pctx = newProfilerContext()
		ctx = context.WithValue(ctx, profilerContextKey{}, pctx)
	}
	if pctx.untagCtx == nil {
		pctx.untagCtx = ctx
	}
	pctx.tags = append(pctx.tags, tags...)
	// do not return pprof ctx
	pprof.SetGoroutineLabels(pprof.WithLabels(context.Background(), pprof.Labels(pctx.tags...)))
	return ctx
}

func (p *profiler) Untag(ctx context.Context) {
	pctx, ok := ctx.Value(profilerContextKey{}).(*profilerContext)
	if ok && pctx.untagCtx != nil {
		pprof.SetGoroutineLabels(pctx.untagCtx)
	} else {
		pprof.SetGoroutineLabels(context.Background())
	}
}

func (p *profiler) Lookup(ctx context.Context, key string) (string, bool) {
	return pprof.Label(ctx, key)
}

func (p *profiler) startProfile() error {
	p.data.Reset()
	return pprof.StartCPUProfile(&p.data)
}

func (p *profiler) stopProfile() {
	pprof.StopCPUProfile()
}

func (p *profiler) analyse() ([]*TagsProfile, error) {
	// parse protobuf data
	pf, err := profile.ParseData(p.data.Bytes())
	if err != nil {
		return nil, err
	}

	// filter cpu value index
	sampleIdx := -1
	for idx, st := range pf.SampleType {
		switch st.Type {
		case "cycles":
			sampleIdx = idx
		case "cpu":
			sampleIdx = idx
		}
		if st.Type == "cpu" {
			sampleIdx = idx
			break
		}
	}
	if sampleIdx < 0 {
		return nil, errors.New("sample type not found")
	}

	// calculate every sample expense
	counter := map[string]*TagsProfile{} // map[tagsKey]funcProfile
	var total int64
	for _, sm := range pf.Sample {
		value := sm.Value[sampleIdx]
		tags := labelToTags(sm.Label)
		tagsKey := tagsToKey(tags)
		tp, ok := counter[tagsKey]
		if !ok {
			tp = &TagsProfile{}
			counter[tagsKey] = tp
			tp.Key = tagsKey
			tp.Tags = tags
		}
		tp.Value += value
		total += value
	}

	// flat to tagsProfiles
	profiles := make([]*TagsProfile, 0, len(counter))
	for _, l := range counter {
		l.Percent = float64(l.Value) / float64(total)
		profiles = append(profiles, l)
	}
	return profiles, nil
}

type TagsProfile struct {
	Key     string
	Tags    []string
	Value   int64
	Percent float64 // <= 1.0
}

func labelToTags(label map[string][]string) []string {
	tags := make([]string, 0, len(label)*2)
	for k, v := range label {
		tags = append(tags, k, strings.Join(v, ","))
	}
	return tags
}

func tagsToKey(tags []string) string {
	if len(tags)%2 != 0 {
		return ""
	}
	tagsPair := make([]string, 0, len(tags)/2)
	for i := 0; i < len(tags); i += 2 {
		tagsPair = append(tagsPair, fmt.Sprintf("%s=%s", tags[i], tags[i+1]))
	}
	// sort tags to make it a unique key
	sort.Strings(tagsPair)
	return strings.Join(tagsPair, "|")
}
