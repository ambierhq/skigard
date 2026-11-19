#!/usr/bin/env bash
# Smoke test of the packed npm tarballs for THIS machine's platform, as a user would use them.
#   scripts/npm-smoke.sh DIST VERSION      (DIST = GoReleaser output, VERSION = its version without "v")
# Builds the packages with npm-pack.mjs, installs the main and platform tarball into a scratch project, then checks
# `npx skigard version`, exit code propagation and `approve` through a real pty.
set -euo pipefail
cd "$(dirname "$0")/.."
dist="${1:?usage: npm-smoke.sh DIST VERSION}"; version="${2:?usage: npm-smoke.sh DIST VERSION}"
W="$(mktemp -d)"; trap 'rm -rf "$W"' EXIT
fail() { echo "FAIL: $*" >&2; exit 1; }
for v in $(env | grep -E '^(PI_|CLAUDE|CODEX|GEMINI|OPENCODE|CURSOR|GOOSE|AMP_)' | cut -d= -f1); do unset "$v"; done
export npm_config_update_notifier=false npm_config_cache="$W/npm-cache"

node scripts/npm-pack.mjs --dist "$dist" --version "$version" --out "$W/out" --pack >/dev/null
os="$(node -p 'process.platform')"; cpu="$(node -p 'process.arch')"
main="$(ls "$W"/out/tarballs/skigard-*.tgz)"; plat="$(ls "$W/out/tarballs/ambierhq-skigard-${os}-${cpu}"-*.tgz)"

mkdir "$W/proj" && cd "$W/proj" && npm init -y >/dev/null
npm install --no-audit --no-fund --ignore-scripts "$plat" "$main" >/dev/null
grep -q '"postinstall"' node_modules/skigard/package.json && fail "main package has a postinstall"
out="$(npx --no-install skigard version)"; [[ "$out" == "skigard $version"* ]] || fail "version output: $out"
npx --no-install skigard scan definitely-not-a-skill >/dev/null 2>&1 && fail "scan of a missing skill should exit non-zero"
code=0; SKIGARD_HOME="$W/h" npx --no-install skigard scan nope >/dev/null 2>&1 || code=$?
[ "$code" = 2 ] || fail "exit code not propagated: $code"

export SKIGARD_HOME="$W/home" HOME="$W/user"; mkdir -p "$HOME" "$W/src/good"
printf -- '---\nname: good\ndescription: formats text\n---\n# good\nprose only\n' > "$W/src/good/SKILL.md"
shim="$W/proj/node_modules/.bin/skigard"
"$shim" init >/dev/null; "$shim" add "$W/src" --all >/dev/null
cd - >/dev/null
res=$(python3 scripts/pty_approve.py "$shim" "$SKIGARD_HOME" good nope); [ "$(echo "$res" | head -1)" = 3 ] || fail "wrong answer: $res"
res=$(python3 scripts/pty_approve.py "$shim" "$SKIGARD_HOME" good good); [ "$(echo "$res" | head -1)" = 0 ] || fail "right answer: $res"
echo "npm smoke ok ($os-$cpu, $version)"
