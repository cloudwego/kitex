/*
 * Copyright 2023 CloudWeGo Authors
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

package thriftreflection

// CheckMethodIDLConsistency
func CheckMethodIDLConsistency(localMethod *MethodDescriptor, localSvcDescriptor, remoteSvcDescriptor *ServiceDescriptor) bool {
	// try to find remote method by method name
	var remoteMethod *MethodDescriptor
	for _, rm := range remoteSvcDescriptor.Methods {
		if rm.MethodName == localMethod.MethodName {
			remoteMethod = rm
			break
		}
	}

	if remoteMethod == nil {
		return false
	}

	// compare signature
	if localMethod.MethodSignature != remoteMethod.MethodSignature {
		return false
	}

	// check included structs number
	if len(localMethod.IncludedStructs) != len(remoteMethod.IncludedStructs) {
		return false
	}

	// check each struct descriptor
	for structFullName := range localMethod.IncludedStructs {
		localStruct := localSvcDescriptor.UsedStructs[structFullName]
		var remoteStruct *StructDescriptor
		for _, rs := range remoteSvcDescriptor.UsedStructs {
			if rs.StructName == localStruct.StructName {
				remoteStruct = rs
				break
			}
		}
		if remoteStruct == nil {
			return false
		}

		if localStruct.StructSignature != remoteStruct.StructSignature {
			return false
		}
	}
	return true
}
