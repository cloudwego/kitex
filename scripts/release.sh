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
# Kitex Release Script
#
# This script creates a new release tag for the Kitex project.
# It performs the following steps:
#   1. Validates you're on the main branch and up-to-date with origin
#   2. Prompts for the target commit to release
#   3. Shows changes since the last version
#   4. Allows you to choose version bump type (major/minor/patch)
#   5. Validates version.go matches the new version
#   6. Creates and pushes the release tag
#
# Usage:
#   ./release.sh
#
# Prerequisites:
#   - Must be on main branch
#   - Local repo must be up-to-date with origin
#   - Target commit must exist on origin
#
# The script will interactively prompt for:
#   - Target commit hash to release
#   - Version bump type (patch/minor/major)
#   - Confirmation before creating the tag
#

set -e

DEFAULT_BRANCH=main
GIT_ROOT=$(git rev-parse --show-toplevel)
SCRIPTS_ROOT=$GIT_ROOT/scripts
CHECK_VERSION_CMD=$SCRIPTS_ROOT/.utils/check_version.sh
CHECK_GO_MOD_CMD=$SCRIPTS_ROOT/.utils/check_go_mod.sh

# Source shared utility functions
source $SCRIPTS_ROOT/.utils/funcs.sh

echo "🚀 Kitex Release Script"
echo "======================="

# Fetch latest changes from origin
echo "📥 Fetching latest changes from origin..."
git fetch -p --force --tags origin

# 1. Ask user which commit to release
echo
read -p "⌨️  Enter the commit hash you want to release: " target_commit
if [ -z "$target_commit" ]; then
    echo "❌ Error: Commit hash is required"
    exit 1
fi

# Resolve to full commit hash
target_commit=$(git rev-parse "$target_commit")
echo "📌 Target commit: $target_commit"
echo

# 2. Check if commit exists on origin
check_commit_exists "$target_commit" "$DEFAULT_BRANCH"

# 3. Get latest version tag
latest_version=$(get_latest_version)

# 4. Show diff between latest version and target commit
if [ "$latest_version" = "v0.0.0" ]; then
    show_first_release_changes "$target_commit" 10
else
    if ! show_changes "$latest_version" "$target_commit" "Changes since latest version $latest_version"; then
        exit 0
    fi
fi

# Ask user to confirm the changes before proceeding
if ! confirm_changes; then
    exit 1
fi

# 5. Ask user for version bump type

# Calculate what each version would be
major_version=$(increment_version "$latest_version" "major")
minor_version=$(increment_version "$latest_version" "minor")
patch_version=$(increment_version "$latest_version" "patch")

echo
echo "⚙️  Available version bump types:"
echo "  1) patch - for bug fixes ($patch_version)"
echo "  2) minor - for new features ($minor_version)"
echo "  3) major - for breaking changes ($major_version)"
echo

while true; do
    read -p "Select version bump type (1/2/3): " bump_choice
    case $bump_choice in
        1) bump_type="patch"; break;;
        2) bump_type="minor"; break;;
        3) bump_type="major"; break;;
        *) echo "Please enter 1, 2, or 3";;
    esac
done

new_version=$(increment_version "$latest_version" "$bump_type")
echo
echo "New version will be: $new_version"

# Check version.go file
echo
$CHECK_VERSION_CMD "$new_version"

# Check go.mod
echo
$CHECK_GO_MOD_CMD

# Final confirmation
echo
read -p "Create release tag $new_version for commit $target_commit? (y/N): " confirm
if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
    echo "❌ Release cancelled"
    exit 1
fi

# Create and push tag
echo "🏷️  Creating tag $new_version..."
git tag -a "$new_version" "$target_commit" -m "Release $new_version"

echo "📤 Pushing tag to origin..."
git push origin "$new_version"

echo
echo "🎉 Release $new_version created successfully!"
echo "Tag: $new_version"
echo "Commit: $target_commit"
