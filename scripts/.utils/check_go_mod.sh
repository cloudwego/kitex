#!/bin/bash
# Copyright 2025 CloudWeGo Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#
# Go Module Dependency Checker
#
# This script validates that all CloudWeGo dependencies in go.mod use proper
# semantic version tags (vA.B.C format) instead of commit hashes or branch names.
#
# Purpose:
#   - Ensures CloudWeGo dependencies follow proper versioning practices
#   - Validates go.mod contains only formal release versions
#   - Prevents release with non-standard dependency versions
#
# Usage:
#   ./scripts/.utils/check_go_mod.sh
#
# The script:
#   1. Searches go.mod for github.com/cloudwego/* dependencies
#   2. Checks that each dependency uses semantic versioning (vX.Y.Z)
#   3. Reports any dependencies using non-standard versions
#   4. Exits with error if invalid dependencies are found
#
# Called by:
#   - release.sh (before creating release tags)
#   - release-hotfix.sh (before creating hotfix tags)
#

GO_MOD_FILE="go.mod"

cd $(git rev-parse --show-toplevel)

# Check go.mod for cloudwego dependencies using proper tag versions
echo "🔍 Checking go.mod for cloudwego dependencies..."
if [ -f "$GO_MOD_FILE" ]; then
    # Find cloudwego dependencies that don't use formal tag versions (vA.B.C format)
    invalid_deps=$(grep "github.com/cloudwego/" "$GO_MOD_FILE" | \
      grep -v "// indirect" | grep -v "module " | \
      grep -v -E "v[0-9]+\.[0-9]+\.[0-9]+$" | awk '{print $1 " " $2}')

    if [ -n "$invalid_deps" ]; then
        echo "❌ Error: Found cloudwego dependencies not using formal tag versions:"
        echo
        echo "   $invalid_deps"
        echo
        echo "   All github.com/cloudwego/* dependencies must use formal tag versions like v0.1.2 format"
        echo "   Please update go.mod to use proper tagged versions for these dependencies"
        exit 1
    else
        echo "✅ All cloudwego dependencies use formal tag versions"
    fi
else
    echo "⚠️  WARNING: go.mod not found"
fi
