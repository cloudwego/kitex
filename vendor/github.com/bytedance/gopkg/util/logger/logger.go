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
)

// Logger is a logger interface that provides logging function with levels.
type Logger interface {
	Trace(v ...interface{})
	Debug(v ...interface{})
	Info(v ...interface{})
	Notice(v ...interface{})
	Warn(v ...interface{})
	Error(v ...interface{})
	Fatal(v ...interface{})

	Tracef(format string, v ...interface{})
	Debugf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Noticef(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Fatalf(format string, v ...interface{})

	CtxTracef(ctx context.Context, format string, v ...interface{})
	CtxDebugf(ctx context.Context, format string, v ...interface{})
	CtxInfof(ctx context.Context, format string, v ...interface{})
	CtxNoticef(ctx context.Context, format string, v ...interface{})
	CtxWarnf(ctx context.Context, format string, v ...interface{})
	CtxErrorf(ctx context.Context, format string, v ...interface{})
	CtxFatalf(ctx context.Context, format string, v ...interface{})
}

// Level defines the priority of a log message.
// When a logger is configured with a level, any log message with a lower
// log level (smaller by integer comparison) will not be output.
type Level int

// The levels of logs.
const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelNotice
	LevelWarn
	LevelError
	LevelFatal
)

// SetLevel sets the level of logs below which logs will not be output.
// The default log level is LevelTrace.
func SetLevel(lv Level) {
	if lv < LevelTrace || lv > LevelFatal {
		panic("invalid level")
	}
	level = lv
}

// Fatal calls the default logger's Fatal method and then os.Exit(1).
func Fatal(v ...interface{}) {
	defaultLogger.Fatal(v)
}

// Error calls the default logger's Error method.
func Error(v ...interface{}) {
	if level > LevelError {
		return
	}
	defaultLogger.Error(v)
}

// Warn calls the default logger's Warn method.
func Warn(v ...interface{}) {
	if level > LevelWarn {
		return
	}
	defaultLogger.Warn(v)
}

// Notice calls the default logger's Notice method.
func Notice(v ...interface{}) {
	if level > LevelNotice {
		return
	}
	defaultLogger.Notice(v)
}

// Info calls the default logger's Info method.
func Info(v ...interface{}) {
	if level > LevelInfo {
		return
	}
	defaultLogger.Info(v)
}

// Debug calls the default logger's Debug method.
func Debug(v ...interface{}) {
	if level > LevelDebug {
		return
	}
	defaultLogger.Debug(v)
}

// Trace calls the default logger's Trace method.
func Trace(v ...interface{}) {
	if level > LevelTrace {
		return
	}
	defaultLogger.Trace(v)
}

// Fatalf calls the default logger's Fatalf method and then os.Exit(1).
func Fatalf(format string, v ...interface{}) {
	defaultLogger.Fatalf(format, v)
}

// Errorf calls the default logger's Errorf method.
func Errorf(format string, v ...interface{}) {
	if level > LevelError {
		return
	}
	defaultLogger.Errorf(format, v)
}

// Warnf calls the default logger's Warnf method.
func Warnf(format string, v ...interface{}) {
	if level > LevelWarn {
		return
	}
	defaultLogger.Warnf(format, v)
}

// Noticef calls the default logger's Noticef method.
func Noticef(format string, v ...interface{}) {
	if level > LevelNotice {
		return
	}
	defaultLogger.Noticef(format, v)
}

// Infof calls the default logger's Infof method.
func Infof(format string, v ...interface{}) {
	if level > LevelInfo {
		return
	}
	defaultLogger.Infof(format, v)
}

// Debugf calls the default logger's Debugf method.
func Debugf(format string, v ...interface{}) {
	if level > LevelDebug {
		return
	}
	defaultLogger.Debugf(format, v)
}

// Tracef calls the default logger's Tracef method.
func Tracef(format string, v ...interface{}) {
	if level > LevelTrace {
		return
	}
	defaultLogger.Tracef(format, v)
}

// CtxFatalf calls the default logger's CtxFatalf method and then os.Exit(1).
func CtxFatalf(ctx context.Context, format string, v ...interface{}) {
	defaultLogger.CtxFatalf(ctx, format, v)
}

// CtxErrorf calls the default logger's CtxErrorf method.
func CtxErrorf(ctx context.Context, format string, v ...interface{}) {
	if level > LevelError {
		return
	}
	defaultLogger.CtxErrorf(ctx, format, v)
}

// CtxWarnf calls the default logger's CtxWarnf method.
func CtxWarnf(ctx context.Context, format string, v ...interface{}) {
	if level > LevelWarn {
		return
	}
	defaultLogger.CtxWarnf(ctx, format, v)
}

// CtxNoticef calls the default logger's CtxNoticef method.
func CtxNoticef(ctx context.Context, format string, v ...interface{}) {
	if level > LevelNotice {
		return
	}
	defaultLogger.CtxNoticef(ctx, format, v)
}

// CtxInfof calls the default logger's CtxInfof method.
func CtxInfof(ctx context.Context, format string, v ...interface{}) {
	if level > LevelInfo {
		return
	}
	defaultLogger.CtxInfof(ctx, format, v)
}

// CtxDebugf calls the default logger's CtxDebugf method.
func CtxDebugf(ctx context.Context, format string, v ...interface{}) {
	if level > LevelDebug {
		return
	}
	defaultLogger.CtxDebugf(ctx, format, v)
}

// CtxTracef calls the default logger's CtxTracef method.
func CtxTracef(ctx context.Context, format string, v ...interface{}) {
	if level > LevelTrace {
		return
	}
	defaultLogger.CtxTracef(ctx, format, v)
}

var level Level

var strs = []string{
	"[Trace] ",
	"[Debug] ",
	"[Info] ",
	"[Notice] ",
	"[Warn] ",
	"[Error] ",
	"[Fatal] ",
}

func (lv Level) toString() string {
	if lv >= LevelTrace && lv <= LevelFatal {
		return strs[lv]
	}
	return fmt.Sprintf("[?%d] ", lv)
}
