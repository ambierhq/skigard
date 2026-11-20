#!/bin/sh
# Quick start of the published Linux build inside WSL (Ubuntu), used by .github/workflows/windows-smoke.yml.
# usage: wsl-smoke.sh VERSION PINNED(true|false) EXPECTED
# Runs from the repository root (for examples/hello-skill and scripts/pty_approve.py).
set -eu
version="$1"; pinned="$2"; expected="$3"
fail() { echo "FAIL: $*" >&2; exit 1; }

echo "== kernel"; uname -a; grep -i microsoft /proc/version || echo "(no 'microsoft' in /proc/version)"

echo "== install: curl -fsSL https://ambierhq.com/skigard/install.sh | sh"
export SKIGARD_INSTALL_DIR="$HOME/.local/bin"
if [ "$pinned" = true ]; then SKIGARD_VERSION="$version"; export SKIGARD_VERSION; fi
curl -fsSL https://ambierhq.com/skigard/install.sh | sh
PATH="$SKIGARD_INSTALL_DIR:$PATH"; export PATH
skigard version | grep -E "^skigard v?$expected( |\$)" || fail "version is not $expected"

echo "== quick start"
SKIGARD_HOME="$(mktemp -d)/home"; export SKIGARD_HOME
skigard init
skigard add "$PWD/examples/hello-skill"
skigard scan hello-skill | tee /dev/stderr | grep -q PASS || fail "scan did not pass"

res=$(python3 scripts/pty_approve.py "$(command -v skigard)" "$SKIGARD_HOME" hello-skill nope)
[ "$(echo "$res" | head -1)" = 3 ] || fail "wrong answer should exit 3: $res"
res=$(python3 scripts/pty_approve.py "$(command -v skigard)" "$SKIGARD_HOME" hello-skill hello-skill)
[ "$(echo "$res" | head -1)" = 0 ] || fail "right answer should approve: $res"
skigard list | grep -q approved || fail "not approved"
skigard verify --strict

skigard harness add --id wslsmoke --name WslSmoke --global "$(mktemp -d)/skills"
skigard install hello-skill --to wslsmoke
skigard verify --strict
echo "WSL quick start OK"
