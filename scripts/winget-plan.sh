#!/usr/bin/env bash
# Decide what the release workflow's winget job does for a version, without writing anything.
#   scripts/winget-plan.sh [--dry-run] VERSION
# Needs `gh` authenticated (WINGET_TOKEN in CI; any token for a local dry run: public reads only).
# Env: UPSTREAM (default microsoft/winget-pkgs), FORK_OWNER (default ambierhq), PKG_PATH (default manifests/a/Ambier/Skigard).
#
# Prints key=value lines (also appended to $GITHUB_OUTPUT when set):
#   action = create | update | reuse | skip      pr = number of the open PR (update, reuse)
#   branch = our fork branch to push            kind = "New package" | "New version"
#   reason = why
# Rules (microsoft/winget-pkgs wants one open PR per package until it exists upstream):
#   package not on upstream master + our PR open -> update   (push new manifests to that PR's branch, retitle it)
#   package not on upstream master, no PR open    -> create  (kind "New package")
#   package on upstream master, our PR open for this version -> reuse
#   package on upstream master, version already there       -> skip
#   package on upstream master otherwise                    -> create  (kind "New version")
# --dry-run only labels the output (the script never writes); it exists so the workflow and humans call it the same way.
set -euo pipefail

winget_plan() {
  local version="$1"
  local upstream="${UPSTREAM:-microsoft/winget-pkgs}" owner="${FORK_OWNER:-ambierhq}" pkg="${PKG_PATH:-manifests/a/Ambier/Skigard}"
  local prs pr branch exists vexists

  # Open PRs for the package that come from our fork (head owner), found by title.
  prs="$(gh pr list --repo "$upstream" --state open --search "Ambier.Skigard in:title" --limit 50 \
    --json number,title,headRefName,headRepositoryOwner \
    -q "[.[] | select(.headRepositoryOwner.login == \"$owner\" and (.title | test(\"Ambier[.]Skigard\")))] | sort_by(.number)")"

  if gh api "repos/$upstream/contents/$pkg?ref=master" >/dev/null 2>&1; then exists=1; else exists=0; fi
  if [ "$exists" = 1 ]; then
    if gh api "repos/$upstream/contents/$pkg/$version?ref=master" >/dev/null 2>&1; then vexists=1; else vexists=0; fi
  else
    vexists=0
  fi

  if [ "$exists" = 0 ]; then
    pr="$(printf '%s' "$prs" | jq -r '.[-1].number // empty')"
    if [ -n "$pr" ]; then
      branch="$(printf '%s' "$prs" | jq -r '.[-1].headRefName')"
      emit update "$pr" "$branch" "New package" "package is not on $upstream master and PR #$pr is still open: update its branch '$branch' instead of opening a second PR"
    else
      emit create "" "skigard-$version" "New package" "package is not on $upstream master and no PR of $owner is open"
    fi
    return
  fi
  if [ "$vexists" = 1 ]; then
    emit skip "" "skigard-$version" "New version" "version $version is already on $upstream master"
    return
  fi
  pr="$(printf '%s' "$prs" | jq -r --arg b "skigard-$version" '[.[] | select(.headRefName == $b)][0].number // empty')"
  if [ -n "$pr" ]; then
    emit reuse "$pr" "skigard-$version" "New version" "PR #$pr for this version is already open"
  else
    emit create "" "skigard-$version" "New version" "package exists upstream and no PR for $version is open"
  fi
}

emit() {
  local out
  out="action=$1
pr=$2
branch=$3
kind=$4
reason=$5"
  printf '%s\n' "$out"
  if [ -n "${GITHUB_OUTPUT:-}" ] && [ "$DRY_RUN" = 0 ]; then printf '%s\n' "$out" >>"$GITHUB_OUTPUT"; fi
}

DRY_RUN=0
if [ "${1:-}" = "--dry-run" ]; then DRY_RUN=1; shift; fi
version="${1:-}"
case "$version" in
  *[!0-9A-Za-z.+_-]*|"") echo "winget-plan: usage: winget-plan.sh [--dry-run] VERSION" >&2; exit 2 ;;
esac
[ "$DRY_RUN" = 1 ] && echo "dry run: nothing is written"
winget_plan "$version"
