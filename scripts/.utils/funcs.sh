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
# Shared utility functions for release scripts
#

# Function to show changes between two commits/tags
# Usage: show_changes <from_ref> <to_ref> [title]
show_changes() {
    local from_ref=$1
    local to_ref=$2
    local title=${3:-"Changes"}
    
    echo
    echo "📋 $title:"
    echo "$(printf '%*s' ${#title} | tr ' ' '=')"
    
    # Check if there are any commits between the references
    local commit_count=$(git rev-list --count "$from_ref..$to_ref")
    if [ "$commit_count" -eq 0 ]; then
        echo "✅ No new commits. Nothing to release."
        return 1
    fi
    
    # Show changes grouped by author email
    git log --pretty=format:"%ae|%h %s" "$from_ref..$to_ref" | sort | awk -F'|' '{
        if ($1 != prev_email) {
            if (prev_email != "") print ""
            print "👤 " $1 ":"
            prev_email = $1
        }
        print "  " $2
    }'
    
    return 0
}

# Function to show changes for first release (no previous version)
# Usage: show_first_release_changes <commit> [limit]
show_first_release_changes() {
    local commit=$1
    local limit=${2:-10}
    
    echo
    echo "📋 This is the first release - showing recent commits:"
    echo "===================================================="
    
    git log --pretty=format:"%ae|%h %s" -"$limit" "$commit" | sort | awk -F'|' '{
        if ($1 != prev_email) {
            if (prev_email != "") print ""
            print "👤 " $1 ":"
            prev_email = $1
        }
        print "  " $2
    }'
}

# Function to ask user for confirmation of changes
# Usage: confirm_changes
confirm_changes() {
    echo
    echo "📝 Please review the changes above."
    read -p "Do you want to proceed with these changes? (y/N): " confirm_changes
    if [ "$confirm_changes" != "y" ] && [ "$confirm_changes" != "Y" ]; then
        echo "❌ Release cancelled by user"
        return 1
    fi
    return 0
}

# Function to check if commit exists on origin
# Usage: check_commit_exists <commit> <branch>
check_commit_exists() {
    local commit=$1
    local branch=$2
    if ! git cat-file -e "$commit" 2>/dev/null; then
        echo "❌ Error: Commit $commit does not exist locally"
        exit 1
    fi

    if ! git branch -r --contains "$commit" | grep -q "origin/$branch$"; then
        echo "❌ Error: Commit $commit does not exist on origin/$branch"
        exit 1
    fi
}

# Function to get latest version tag
# Usage: get_latest_version
get_latest_version() {
    local latest_tag=$(git tag -l "v*.*.*" | sort -V | tail -n1)
    if [ -z "$latest_tag" ]; then
        echo "v0.0.0"
    else
        echo "$latest_tag"
    fi
}

# Function to get latest patch version for a given minor version
# Usage: get_latest_patch_version <minor_version>
get_latest_patch_version() {
    local minor_version=$1
    # Remove 'v' prefix and '.x' suffix if present
    minor_version=${minor_version#v}
    minor_version=${minor_version%.x}

    local latest_patch=$(git tag -l "v${minor_version}.*" | sort -V | tail -n1)
    if [ -z "$latest_patch" ]; then
        echo "No tags found for minor version v${minor_version}.x"
        return 1
    else
        echo "$latest_patch"
    fi
}

# Function to increment version
# Usage: increment_version <version> <type>
# where type is one of: major, minor, patch
increment_version() {
    local version=$1
    local type=$2

    # Remove 'v' prefix
    version=${version#v}

    IFS='.' read -r major minor patch <<< "$version"

    case $type in
        "major")
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        "minor")
            minor=$((minor + 1))
            patch=0
            ;;
        "patch")
            patch=$((patch + 1))
            ;;
    esac

    echo "v$major.$minor.$patch"
}

# Function to increment patch version (convenience wrapper)
# Usage: increment_patch_version <version>
increment_patch_version() {
    local version=$1
    increment_version "$version" "patch"
}
