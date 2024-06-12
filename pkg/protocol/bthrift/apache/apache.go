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

// Package apache contains codes originally from https://github.com/apache/thrift.
//
// we're planning to get rid of the pkg, here are steps we're going to work on:
//  1. Remove unnecessary dependencies of apache from kitex
//  2. Move all apache dependencies to this pkg, mainly types, interfaces and consts
//     - We may use type alias at the beginning for better compatibility
//     - Mark interfaces as `Deprecated` since we no longer use it in the future, and we have better implementation.
//  3. For internal dependencies of apache, new alternative implementation will be in:
//     - pkg/protocol/bthrift -> low level encoding or decoding bytes
//     - pkg/remote/codec/thrift -> high level interfaces
//  4. Change necessary dependencies to this file, including code generator
//     (After a period of time)
//  5. Remove apache support of code generator (mainly interfaces)
//  6. Remove type alias and move definition to this file. This may causes compatible issues which are expected.
//     - legacy code generator should use legacy version of kitex, then should not have compatibility issue.
//     (After a period of time)
//  7. Remove interfaces like thrift.TProtocol from this file
//  8. Done
//
// Now we're working on step 1 - 4.
package apache
