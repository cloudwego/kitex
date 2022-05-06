// Copyright 2021 ByteDance Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logger

import (
	"context"
	"fmt"
	"log"
	"os"
)

var (
	_ Logger = (*localLogger)(nil)
)

// SetDefaultLogger sets the default logger.
// This is not concurrency safe, which means it should only be called during init.
func SetDefaultLogger(l Logger) {
	if l == nil {
		panic("logger must not be nil")
	}
	defaultLogger = l
}

var defaultLogger Logger = &localLogger{
	logger: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
}

type localLogger struct {
	logger *log.Logger
}

func (ll *localLogger) logf(lv Level, format *string, v ...interface{}) {
	if level > lv {
		return
	}
	msg := lv.toString()
	if format != nil {
		msg += fmt.Sprintf(*format, v...)
	} else {
		msg += fmt.Sprint(v...)
	}
	ll.logger.Output(3, msg)
	if lv == LevelFatal {
		os.Exit(1)
	}
}

func (ll *localLogger) Fatal(v ...interface{}) {
	ll.logf(LevelFatal, nil, v...)
}

func (ll *localLogger) Error(v ...interface{}) {
	ll.logf(LevelError, nil, v...)
}

func (ll *localLogger) Warn(v ...interface{}) {
	ll.logf(LevelWarn, nil, v...)
}

func (ll *localLogger) Notice(v ...interface{}) {
	ll.logf(LevelNotice, nil, v...)
}

func (ll *localLogger) Info(v ...interface{}) {
	ll.logf(LevelInfo, nil, v...)
}

func (ll *localLogger) Debug(v ...interface{}) {
	ll.logf(LevelDebug, nil, v...)
}

func (ll *localLogger) Trace(v ...interface{}) {
	ll.logf(LevelTrace, nil, v...)
}

func (ll *localLogger) Fatalf(format string, v ...interface{}) {
	ll.logf(LevelFatal, &format, v...)
}

func (ll *localLogger) Errorf(format string, v ...interface{}) {
	ll.logf(LevelError, &format, v...)
}

func (ll *localLogger) Warnf(format string, v ...interface{}) {
	ll.logf(LevelWarn, &format, v...)
}

func (ll *localLogger) Noticef(format string, v ...interface{}) {
	ll.logf(LevelNotice, &format, v...)
}

func (ll *localLogger) Infof(format string, v ...interface{}) {
	ll.logf(LevelInfo, &format, v...)
}

func (ll *localLogger) Debugf(format string, v ...interface{}) {
	ll.logf(LevelDebug, &format, v...)
}

func (ll *localLogger) Tracef(format string, v ...interface{}) {
	ll.logf(LevelTrace, &format, v...)
}

func (ll *localLogger) CtxFatalf(ctx context.Context, format string, v ...interface{}) {
	ll.logf(LevelFatal, &format, v...)
}

func (ll *localLogger) CtxErrorf(ctx context.Context, format string, v ...interface{}) {
	ll.logf(LevelError, &format, v...)
}

func (ll *localLogger) CtxWarnf(ctx context.Context, format string, v ...interface{}) {
	ll.logf(LevelWarn, &format, v...)
}

func (ll *localLogger) CtxNoticef(ctx context.Context, format string, v ...interface{}) {
	ll.logf(LevelNotice, &format, v...)
}

func (ll *localLogger) CtxInfof(ctx context.Context, format string, v ...interface{}) {
	ll.logf(LevelInfo, &format, v...)
}

func (ll *localLogger) CtxDebugf(ctx context.Context, format string, v ...interface{}) {
	ll.logf(LevelDebug, &format, v...)
}

func (ll *localLogger) CtxTracef(ctx context.Context, format string, v ...interface{}) {
	ll.logf(LevelTrace, &format, v...)
}
