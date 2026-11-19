Output "scene2.mp4"
Hide
Type "source $DEMO_ROOT/assets/demo/lib/setup.sh agent"
Enter
Sleep 4s
Show
Sleep 600ms
Type "# an agent (CLAUDECODE is set) tries to approve the blocked skill"
Enter
Type "export CLAUDECODE=1"
Enter
Sleep 800ms
Type "skigard approve free-tokens"
Enter
Sleep 1500ms
Type 'echo "exit code: $?"'
Enter
Sleep 2500ms
Type "skigard list"
Enter
Sleep 5s
Screenshot "shot-2-refused.png"
Sleep 2s
