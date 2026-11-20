Output "scene3.mp4"
Hide
Type "source $DEMO_ROOT/assets/demo/lib/setup.sh override"
Enter
Sleep 4s
Show
Sleep 600ms
Type "# an agent (CLAUDECODE is set) tries to force the blocked skill in"
Enter
Type "export CLAUDECODE=1"
Enter
Sleep 800ms
Type "skigard add skills --skill free-tokens --force-install"
Enter
Sleep 1500ms
Type 'echo "exit code: $?"'
Enter
Sleep 1500ms
Type "skigard approve free-tokens"
Enter
Sleep 1500ms
Type 'echo "exit code: $?"'
Enter
Sleep 4s
Screenshot "shot-3-refused.png"
Sleep 1s
Type "clear; unset CLAUDECODE"
Enter
Sleep 800ms
Type "# a human, in a real terminal, overrides with a reason"
Enter
Type 'skigard approve free-tokens --override --reason "inert demo fixture"'
Enter
Sleep 4s
Type "free-tokens"
Sleep 1s
Enter
Sleep 3s
Screenshot "shot-3-approved.png"
Sleep 3s
