#!/usr/bin/env bash
# Set Pulse's version everywhere it is recorded, optionally on a new release or
# hotfix branch.
#
#   scripts/set-version.sh 0.2.0              # update the files on this branch
#   scripts/set-version.sh 0.2.0 --release    # cut release/v0.2.0 here, then update
#   scripts/set-version.sh 0.1.1 --hotfix     # cut hotfix/v0.1.1 here, then update
#
# Files: VERSION (the source of truth, linked into the Go binaries and the
# dashboard at build time) and the frontend's package.json and
# package-lock.json. CI rejects a release PR where these disagree.
# Nothing is committed.
set -euo pipefail

usage() {
  sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

[ $# -ge 1 ] || usage 1
case "$1" in -h|--help) usage ;; esac

version="${1#v}"
kind=""
case "${2:-}" in
  "") ;;
  --release) kind=release ;;
  --hotfix) kind=hotfix ;;
  *) echo "Unknown option: $2" >&2; usage 1 ;;
esac

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Version must be X.Y.Z (e.g. 0.2.0), got '$1'." >&2
  exit 1
fi

cd "$(git rev-parse --show-toplevel)"

current=$(tr -d '[:space:]' < VERSION 2>/dev/null || echo none)

if git rev-parse -q --verify "refs/tags/v$version" >/dev/null ||
   git ls-remote --exit-code --tags origin "refs/tags/v$version" >/dev/null 2>&1; then
  echo "v$version is already released (the tag exists). Pick a new version." >&2
  exit 1
fi

if [ -n "$kind" ]; then
  branch="$kind/v$version"
  if git rev-parse -q --verify "refs/heads/$branch" >/dev/null; then
    echo "Branch $branch already exists." >&2
    exit 1
  fi
  git switch -c "$branch"
fi

printf '%s\n' "$version" > VERSION
(cd frontend && npm version "$version" --no-git-tag-version --allow-same-version --ignore-scripts >/dev/null)

echo "Version $current -> $version"
for f in VERSION frontend/package.json frontend/package-lock.json; do
  echo "  $f"
done
echo
echo "Next: git commit -am \"chore: version $version\""
if [ -n "$kind" ]; then
  echo "      git push -u origin $branch, then open a PR into master."
fi
