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
