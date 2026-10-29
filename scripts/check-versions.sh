#!/bin/sh
# Checks the version is the same everywhere it's written, and (in CI) that REPO names this
# repository. Usage: scripts/check-versions.sh [tag] [owner/repo]
set -e
cd "$(dirname "$0")/.."
v=$(tr -d ' \r\n' < skills/codedocs/VERSION)
p=$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' .claude-plugin/plugin.json)
fail=0
[ "$v" = "$p" ] || { echo "skills/codedocs/VERSION is $v but .claude-plugin/plugin.json says $p"; fail=1; }
grep -Eq "^## $v( |$)" skills/codedocs/CHANGELOG.md || { echo "skills/codedocs/CHANGELOG.md has no '## $v' entry"; fail=1; }
if [ -n "$1" ] && [ "$1" != "v$v" ]; then echo "tag $1 doesn't match VERSION $v (expected v$v)"; fail=1; fi
if [ -n "$2" ]; then
  r=$(tr -d ' \r\n' < skills/codedocs/REPO)
  [ "$r" = "$2" ] || { echo "skills/codedocs/REPO is '$r' but this repository is '$2': installed copies would download from the wrong place"; fail=1; }
fi
[ $fail = 0 ] && echo "versions ok: $v"
exit $fail
