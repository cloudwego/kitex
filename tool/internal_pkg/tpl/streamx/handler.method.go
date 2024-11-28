package streamx

var HandlerMethodsTpl = `{{define "HandlerMethod"}}
{{- $protocol := .Protocol | getStreamxRef}}
{{- range .AllMethods}}
{{- $unary := and (not .ServerStreaming) (not .ClientStreaming)}}
{{- $clientSide := and .ClientStreaming (not .ServerStreaming)}}
{{- $serverSide := and (not .ClientStreaming) .ServerStreaming}}
{{- $bidiSide := and .ClientStreaming .ServerStreaming}}
{{- $arg := index .Args 0}}
func (s *{{.ServiceName}}Impl) {{.Name}}{{- if $unary}}(ctx context.Context, req {{$arg.Type}}) (resp {{.Resp.Type}}, err error) {
             {{- else if $clientSide}}(ctx context.Context, stream streamx.ClientStreamingServer[{{NotPtr $arg.Type}}, {{NotPtr .Resp.Type}}]) (resp {{.Resp.Type}}, err error) {
             {{- else if $serverSide}}(ctx context.Context, req {{$arg.Type}}, stream streamx.ServerStreamingServer[{{NotPtr .Resp.Type}}]) (err error) {
             {{- else if $bidiSide}}(ctx context.Context, stream streamx.BidiStreamingServer[{{NotPtr $arg.Type}}, {{NotPtr .Resp.Type}}]) (err error) {
             {{- end}}
    // TODO: Your code here...
    return
}
{{- end}}
{{end}}{{/* define "HandlerMethod" */}}
`
