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

package thrift

import (
	"github.com/cloudwego/gopkg/protocol/thrift/base"
)

// Base ...
// Deprecated: use github.com/cloudwego/gopkg/protocol/thrift/base
type Base = base.Base

// NewBase ...
// Deprecated: use github.com/cloudwego/gopkg/protocol/thrift/base
func NewBase() *Base {
	return base.NewBase()
}

// BaseResp ...
// Deprecated: use github.com/cloudwego/gopkg/protocol/thrift/base
type BaseResp = base.BaseResp

// NewBaseResp ...
// Deprecated: use github.com/cloudwego/gopkg/protocol/thrift/base
func NewBaseResp() *BaseResp {
	return base.NewBaseResp()
}

// MergeBase mainly take frameworkBase and only merge the frkBase.Extra with the jsonBase.Extra
//
// NOTICE: this logic must be aligned with mergeBaseAny
func MergeBase(jsonBase, frameworkBase Base) Base {
	if jsonBase.Extra != nil {
		extra := jsonBase.Extra
		for k, v := range frameworkBase.Extra {
			extra[k] = v
		}
		frameworkBase.Extra = extra
	}
	return frameworkBase
}

// mergeBaseAny mainly take frkBase and only merge the frkBase.Extra with the jsonBase.Extra
//
// NOTICE: this logic must be aligned with MergeBase
func mergeBaseAny(jsonBase any, frkBase *Base) *Base {
	if st, ok := jsonBase.(map[string]any); ok {
		// copy from user's Extra
		if ext, ok := st["Extra"]; ok {
			switch v := ext.(type) {
			case map[string]any:
				// from http json
				for key, value := range v {
					if _, ok := frkBase.Extra[key]; !ok {
						if vStr, ok := value.(string); ok {
							if frkBase.Extra == nil {
								frkBase.Extra = map[string]string{}
							}
							frkBase.Extra[key] = vStr
						}
					}
				}
			case map[any]any:
				// from struct map
				for key, value := range v {
					if kStr, ok := key.(string); ok {
						if _, ok := frkBase.Extra[kStr]; !ok {
							if vStr, ok := value.(string); ok {
								if frkBase.Extra == nil {
									frkBase.Extra = map[string]string{}
								}
								frkBase.Extra[kStr] = vStr
							}
						}
					}
				}
			}
		}
	}
	return frkBase
}
