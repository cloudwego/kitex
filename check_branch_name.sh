#!/usr/bin/env bash

basic="(master|develop|main)"
lints="((build|ci|chore|docs|feat|(hot)?fix|perf|refactor|revert|style|test)/.+)"
other="((release/v[0-9]+\.[0-9]+\.[0-9]+(-[a-z0-9.]+(\+[a-z0-9.]+)?)?)|revert-[a-z0-9]+)"
pattern="^(${basic}|${lints}|${other})$"

current=$(git status | head -n1 | sed 's/On branch //')
name=${1:-$current}
if [[ ! $name =~ $pattern ]]; then
    echo "branch name '$name' is invalid"
    exit 1
else
    echo "branch name '$name' is valid"
fi
