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
