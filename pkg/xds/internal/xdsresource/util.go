/*
 * Copyright 2021 CloudWeGo Authors
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

package xdsresource

import (
	"fmt"
	"strings"
)

func processUnmarshalErrors(errs []error, errMap map[string]error) error {
	var b strings.Builder
	for _, err := range errs {
		b.WriteString(err.Error())
		b.WriteString("\n")
	}

	if len(errMap) > 0 {
		b.WriteString("Error per resource:\n")
		for name, err := range errMap {
			b.WriteString(fmt.Sprintf("resource %s: %s;\n", name, err.Error()))
		}
	}

	return fmt.Errorf(b.String())
}

func combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	var b strings.Builder
	for _, err := range errs {
		b.WriteString(err.Error())
		b.WriteString("\n")
	}
	return fmt.Errorf(b.String())
}
