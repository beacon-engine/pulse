#!/usr/bin/env bash
# Point git at the version-controlled hooks in .githooks/.
# Run once per clone (from anywhere in the repo).
set -e

repo_root="$(git rev-parse --show-toplevel)"
git -C "$repo_root" config core.hooksPath .githooks
chmod +x "$repo_root"/.githooks/* 2>/dev/null || true

echo "✅ core.hooksPath set to .githooks — shared git hooks are now active."
