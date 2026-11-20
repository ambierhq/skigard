# Sourced inside the recorded shell (hidden part of each tape). Never run on a developer shell by accident:
# it needs DEMO_ROOT, DEMO_BIN and DEMO_TMP, which scripts/demo.sh sets.
: "${DEMO_ROOT:?run through scripts/demo.sh}" "${DEMO_BIN:?}" "${DEMO_TMP:?}"
# Remove every agent marker so the human scenes are honest; scene 3 sets CLAUDECODE=1 on screen.
for v in $(env | cut -d= -f1 | grep -E '^(PI_|CLAUDE|CODEX|GEMINI|OPENCODE|CURSOR|GOOSE|AMP_|ORCA)'); do unset "$v"; done
export SKIGARD_HOME="$DEMO_TMP/home-$1"
export PATH="$DEMO_BIN:$PATH"
export PS1='\[\e[38;2;${BRAND_ACCENT_RGB}m\]\$\[\e[0m\] '
export HOME="$DEMO_TMP/fakehome"; mkdir -p "$HOME"
rm -rf "$SKIGARD_HOME"
rm -rf "$DEMO_TMP/work"; mkdir -p "$DEMO_TMP/work"; cp -R "$DEMO_ROOT/assets/demo/fixtures/skills" "$DEMO_TMP/work/skills"
cd "$DEMO_TMP/work"
skigard init >/dev/null
# `add` runs the real decision in auto mode (default): commit-helper is installed, free-tokens stays in quarantine.
case "$1" in
  blocked)  skigard add skills --skill commit-helper >/dev/null ;;
  override) skigard add skills --all >/dev/null 2>&1 ;;
  tamper)   skigard add skills --skill commit-helper >/dev/null ;;
esac
clear
