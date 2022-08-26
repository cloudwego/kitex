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

import _ "embed"

// BootstrapTpl is the template for generating bootstrap.sh.
//
//go:embed bootstrap.sh.tmpl
var BootstrapTpl string

// BuildTpl is the template for generating build.sh.
//
//go:embed build.sh.tmpl
var BuildTpl string

// ClientTpl is the template for generating client.go.
//
//go:embed client.go.tmpl
var ClientTpl string

// HandlerTpl is the template for generating handler.go.
//
//go:embed handler.go.tmpl
var HandlerTpl string

// HandlerMethodTpl is the template for generating methods in handler.go.
//
//go:embed handler.method.tmpl
var HandlerMethodsTpl string

// InvokerTpl is the template for generating invoker.go.
//
//go:embed invoker.go.tmpl
var InvokerTpl string

// MainTpl is the template for generating main.go.
//
//go:embed main.go.tmpl
var MainTpl string

// ServerTpl is the template for generating server.go.
//
//go:embed server.go.tmpl
var ServerTpl string

// ServiceTpl is the template for generating the servicename.go source.
//
//go:embed service.go.tmpl
var ServiceTpl string
