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
# Version Consistency Checker
#
# This script validates that the version in version.go matches the intended
# release version to ensure consistency across the codebase.
#
# Purpose:
#   - Prevents releasing with mismatched version numbers
#   - Ensures version.go is updated before creating release tags
#   - Provides interactive confirmation for version mismatches
#
# Usage:
#   ./scripts/.utils/check_version.sh <target_version>
#
# Parameters:
#   target_version - The version being released (e.g., v1.2.3)
#
# The script:
#   1. Reads the Version constant from version.go
#   2. Compares it with the target release version
#   3. Warns if versions don't match and prompts for confirmation
#   4. Exits with error if user chooses not to continue
#
# Called by:
#   - release.sh (before creating release tags)
#   - release-hotfix.sh (before creating hotfix tags)
#

VERSION_FILE="version.go"
new_version=$1

if [ -z "$new_version" ]; then
    echo "❌ Error: Version parameter is required"
    echo "Usage: $0 <version>"
    exit 1
fi

cd $(git rev-parse --show-toplevel)

if [ -f "$VERSION_FILE" ]; then
  version_go_version=$(grep -o 'Version = "v[^"]*"' $VERSION_FILE | sed 's/Version = "\([^"]*\)"/\1/')
  if [ "$version_go_version" != "$new_version" ]; then
    echo "⚠️  WARNING: $VERSION_FILE contains $version_go_version but you're releasing $new_version"
    echo "   Please update $VERSION_FILE to match the release version."
    read -p "   Continue anyway? (y/N): " continue_anyway
    if [ "$continue_anyway" != "y" ] && [ "$continue_anyway" != "Y" ]; then
      echo "❌ Release cancelled"
      exit 1
    fi
  else
    echo "✅ $VERSION_FILE matches release version: $new_version"
  fi
else
  echo "⚠️  WARNING: $VERSION_FILE not found"
fi
