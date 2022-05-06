/*
 * Copyright 2022 ByteDance Inc.
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

package utils

import (
    `fmt`
    `reflect`
)

type TypeError struct {
    Note string
    Type reflect.Type
}

func (self TypeError) Error() string {
    if self.Note != "" {
        return fmt.Sprintf("TypeError(%s): %s", self.Type, self.Note)
    } else {
        return fmt.Sprintf("TypeError(%s): not supported by Thrift", self.Type)
    }
}

type SyntaxError struct {
    Pos    int
    Src    string
    Reason string
}

func (self SyntaxError) Error() string {
    return fmt.Sprintf("Syntax error at position %d: %s", self.Pos, self.Reason)
}

func EType(vt reflect.Type) TypeError {
    return TypeError {
        Type: vt,
    }
}

func ESyntax(pos int, src string, reason string) SyntaxError {
    return SyntaxError {
        Pos    : pos,
        Src    : src,
        Reason : reason,
    }
}

func ESetList(pos int, src string, vt reflect.Type) SyntaxError {
    return ESyntax(pos, src, fmt.Sprintf(`ambiguous type between set<%s> and list<%s>, please specify in the "frugal" tag`, vt, vt))
}

func ENotSupp(vt reflect.Type, alt string) TypeError {
    return TypeError {
        Type: vt,
        Note: fmt.Sprintf("Thrift does not support %s, use %s instead", vt, alt),
    }
}
