#! /bin/bash

COPYRIGHT_HEADER='/*
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

'

kitex -thrift no_default_serdes -module github.com/cloudwego/kitex  -gen-path .. ./mock.thrift

kitex -thrift no_default_serdes -module github.com/cloudwego/kitex -gen-path .. ./testservice.thrift

rm -rf ./mock # not in use, rm it

# Add copyright header to all generated .go files under thrift/ (including subdirs)
find . -name '*.go' -type f | while read f; do
    if ! head -1 "$f" | grep -q 'Copyright'; then
        printf '%s%s' "$COPYRIGHT_HEADER" "$(cat "$f")" > "$f"
    fi
done

gofmt -w .
