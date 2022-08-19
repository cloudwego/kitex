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

//go:embed bootstrap.sh.tmpl
// BootstrapTpl is the template for generating bootstrap.sh.
var BootstrapTpl string

//go:embed build.sh.tmpl
// BuildTpl is the template for generating build.sh.
var BuildTpl string

//go:embed client.go.tmpl
// ClientTpl is the template for generating client.go.
var ClientTpl string

//go:embed handler.go.tmpl
// HandlerTpl is the template for generating handler.go.
var HandlerTpl string

//go:embed handler.method.tmpl
// HandlerMethodTpl is the template for generating methods in handler.go.
var HandlerMethodsTpl string

//go:embed invoker.go.tmpl
// InvokerTpl is the template for generating invoker.go.
var InvokerTpl string

//go:embed main.go.tmpl
// MainTpl is the template for generating main.go.
var MainTpl string

//go:embed server.go.tmpl
// ServerTpl is the template for generating server.go.
var ServerTpl string

//go:embed service.go.tmpl
// ServiceTpl is the template for generating the servicename.go source.
var ServiceTpl string
