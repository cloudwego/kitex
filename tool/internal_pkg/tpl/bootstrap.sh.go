// Copyright 2022 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tpl

// BootstrapTpl is the template for generating bootstrap.sh.
var BootstrapTpl string = `#! /usr/bin/env bash
CURDIR=$(cd $(dirname $0); pwd)

if [ "X$1" != "X" ]; then
    RUNTIME_ROOT=$1
else
    RUNTIME_ROOT=${CURDIR}
fi

export KITEX_RUNTIME_ROOT=$RUNTIME_ROOT
export KITEX_LOG_DIR="$RUNTIME_ROOT/log"

if [ ! -d "$KITEX_LOG_DIR/app" ]; then
    mkdir -p "$KITEX_LOG_DIR/app"
fi

if [ ! -d "$KITEX_LOG_DIR/rpc" ]; then
    mkdir -p "$KITEX_LOG_DIR/rpc"
fi

exec "$CURDIR/bin/{{.RealServiceName}}"
`
