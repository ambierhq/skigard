#!/usr/bin/env bash
# Tests for scripts/winget-plan.sh with a stubbed `gh` (no network). Run: scripts/test-winget-plan.sh
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/bin"
# The stub answers from files in $STUB: prs.json (raw `gh pr list --json` output; the stub applies the -q filter with jq), pkg (exists if present), versions (one per line).
cat > "$tmp/bin/gh" <<'STUB'
#!/usr/bin/env bash
case "$1" in
  pr) q=""; while [ $# -gt 0 ]; do [ "$1" = "-q" ] && q="$2"; shift; done; jq -c "$q" "$STUB/prs.json" ;;
  api)
    case "$2" in
      */contents/manifests/a/Ambier/Skigard\?ref=master) [ -e "$STUB/pkg" ] ;;
      */contents/manifests/a/Ambier/Skigard/*\?ref=master)
        v="${2#*Skigard/}"; v="${v%%\?*}"; grep -qx "$v" "$STUB/versions" 2>/dev/null ;;
      *) exit 1 ;;
    esac ;;
  *) echo "unexpected gh $*" >&2; exit 1 ;;
esac
STUB
chmod +x "$tmp/bin/gh"
export PATH="$tmp/bin:$PATH" STUB="$tmp/stub"
fails=0

# case NAME VERSION EXPECTED_ACTION EXPECTED_PR PKG_EXISTS(0|1) VERSIONS PRS_JSON
run() {
  local name="$1" version="$2" action="$3" pr="$4" pkg="$5" versions="$6" prs="$7"
  rm -rf "$STUB"; mkdir "$STUB"
  [ "$pkg" = 1 ] && : > "$STUB/pkg"
  printf '%s\n' "$versions" > "$STUB/versions"
  printf '%s' "$prs" > "$STUB/prs.json"
  out="$("$here/winget-plan.sh" --dry-run "$version")"
  got_action="$(printf '%s\n' "$out" | sed -n 's/^action=//p')"
  got_pr="$(printf '%s\n' "$out" | sed -n 's/^pr=//p')"
  if [ "$got_action" = "$action" ] && [ "$got_pr" = "$pr" ]; then echo "ok   $name"; else echo "FAIL $name: got action=$got_action pr=$got_pr, want $action $pr"; fails=$((fails+1)); fi
}

open_pr='[{"number":7,"title":"New package: Ambier.Skigard version 0.1.2","headRefName":"skigard-0.1.2","headRepositoryOwner":{"login":"ambierhq"}}]'
other_pr='[{"number":9,"title":"New package: Ambier.Skigard version 0.1.2","headRefName":"skigard-0.1.2","headRepositoryOwner":{"login":"someone"}}]'
run "no package, no PR -> create" 0.1.2 create "" 0 "" '[]'
run "no package, open PR -> update it" 0.1.3 update 7 0 "" "$open_pr"
run "no package, open PR, same version -> update (idempotent)" 0.1.2 update 7 0 "" "$open_pr"
run "no package, someone else's PR -> create" 0.1.2 create "" 0 "" "$other_pr"
run "package exists, version missing, no PR -> create" 0.1.3 create "" 1 "0.1.2" '[]'
run "package exists, open PR for another version -> create" 0.1.3 create "" 1 "0.1.1" "$open_pr"
run "package exists, PR for this version open -> reuse" 0.1.2 reuse 7 1 "0.1.1" "$open_pr"
run "package exists, version already merged -> skip" 0.1.2 skip "" 1 "0.1.2" '[]'
if "$here/winget-plan.sh" 'bad version;' >/dev/null 2>&1; then echo "FAIL invalid version accepted"; fails=$((fails+1)); else echo "ok   invalid version rejected"; fi
exit $((fails > 0))
