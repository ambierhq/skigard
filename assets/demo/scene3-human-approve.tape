Output "scene3.mp4"
Hide
Type "source $DEMO_ROOT/assets/demo/lib/setup.sh human"
Enter
Sleep 4s
Show
Sleep 600ms
Type "# a human, in a real terminal, approves the benign skill"
Enter
Type "skigard approve commit-helper"
Enter
Sleep 4s
Type "commit-helper"
Sleep 1s
Enter
Sleep 3s
Screenshot "shot-3-approved.png"
Sleep 1s
Type "clear; skigard list"
Enter
Sleep 2s
Type "grep -E 'name =|content_hash|by =|at =' $SKIGARD_HOME/skigard.lock"
Enter
Sleep 5s
