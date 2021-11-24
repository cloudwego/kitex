/*
 * Copyright 2021 CloudWeGo Authors
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

package klog

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

var (
	_ Logger       = (*localLogger)(nil)
	_ FormatLogger = (*localLogger)(nil)
)

// SetOutput sets the output of default logger. By default, it is stderr.
func SetOutput(w io.Writer) {
	defaultLogger.logger.SetOutput(w)
}

// DefaultLogger return the default logger for kitex.
func DefaultLogger() FormatLogger {
	return defaultLogger
}

// Fatal calls the default logger's Fatal method and then os.Exit(1).
func Fatal(v ...interface{}) {
	defaultLogger.logf(LevelFatal, nil, v...)
}

// Error calls the default logger's Error method.
func Error(v ...interface{}) {
	defaultLogger.logf(LevelError, nil, v...)
}

// Warn calls the default logger's Warn method.
func Warn(v ...interface{}) {
	defaultLogger.logf(LevelWarn, nil, v...)
}

// Notice calls the default logger's Notice method.
func Notice(v ...interface{}) {
	defaultLogger.logf(LevelNotice, nil, v...)
}

// Info calls the default logger's Info method.
func Info(v ...interface{}) {
	defaultLogger.logf(LevelInfo, nil, v...)
}

// Debug calls the default logger's Debug method.
func Debug(v ...interface{}) {
	defaultLogger.logf(LevelDebug, nil, v...)
}

// Trace calls the default logger's Trace method.
func Trace(v ...interface{}) {
	defaultLogger.logf(LevelTrace, nil, v...)
}

// Fatalf calls the default logger's Fatalf method and then os.Exit(1).
func Fatalf(format string, v ...interface{}) {
	defaultLogger.logf(LevelFatal, &format, v...)
}

// Errorf calls the default logger's Errorf method.
func Errorf(format string, v ...interface{}) {
	defaultLogger.logf(LevelError, &format, v...)
}

// Warnf calls the default logger's Warnf method.
func Warnf(format string, v ...interface{}) {
	defaultLogger.logf(LevelWarn, &format, v...)
}

// Noticef calls the default logger's Noticef method.
func Noticef(format string, v ...interface{}) {
	defaultLogger.logf(LevelNotice, &format, v...)
}

// Infof calls the default logger's Infof method.
func Infof(format string, v ...interface{}) {
	defaultLogger.logf(LevelInfo, &format, v...)
}

// Debugf calls the default logger's Debugf method.
func Debugf(format string, v ...interface{}) {
	defaultLogger.logf(LevelDebug, &format, v...)
}

// Tracef calls the default logger's Tracef method.
func Tracef(format string, v ...interface{}) {
	defaultLogger.logf(LevelTrace, &format, v...)
}

var defaultLogger = &localLogger{
	logger: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
	pool: &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	},
}

type localLogger struct {
	logger *log.Logger
	pool   *sync.Pool
}

func (ll *localLogger) logf(lv Level, format *string, v ...interface{}) {
	if level > lv {
		return
	}
	buf := ll.pool.Get().(*bytes.Buffer)
	buf.WriteString(lv.toString())
	if format != nil {
		buf.WriteString(fmt.Sprintf(*format, v...))
	} else {
		buf.WriteString(fmt.Sprint(v...))
	}
	ll.logger.Output(3, buf.String())
	buf.Reset()
	ll.pool.Put(buf)
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
