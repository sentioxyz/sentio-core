#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "usage: git dev <branch-name>" >&2
  exit 2
fi

BRANCH=dev/$(git config --get user.email | cut -d @ -f1)/$1

# Refuse to run on a dirty working tree. The branch-creation path below does
# `git reset --hard origin/main`, which would silently discard uncommitted
# work. Stash or commit first.
if ! git diff-index --quiet HEAD -- || [ -n "$(git ls-files --others --exclude-standard)" ]; then
  echo "error: working tree is not clean — stash or commit before running 'git dev'" >&2
  echo "       (this alias runs 'git reset --hard origin/main' on new-branch creation)" >&2
  git status --short >&2
  exit 1
fi

git checkout "$BRANCH" || (git checkout -b "$BRANCH" && git fetch && git reset --hard origin/main)
