/*
 * Copyright 2025 CloudWeGo Authors
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

/*
Package types provides common type definitions for streaming to avoid circular dependencies.

# Background

This package was created to resolve circular dependency issues between packages.
The circular dependency path was:
  - pkg/streaming imports pkg/kerrors
  - pkg/kerrors imports pkg/remote/trans/nphttp2/status
  - pkg/remote/trans/nphttp2/status need to use streaming-related types

If common streaming types (such as TimeoutType) were defined in pkg/streaming,
it would create a circular dependency: pkg/streaming ↔ pkg/remote/trans/nphttp2/status.

# Solution

By extracting shared streaming type definitions into pkg/streaming/types:
  - pkg/streaming can import pkg/streaming/types
  - pkg/remote/trans/* packages can import pkg/streaming/types
  - The circular dependency is broken

# Convention

All future common streaming type definitions should be placed in this package
to maintain clean dependency relationships and avoid circular dependencies.
*/
package types
