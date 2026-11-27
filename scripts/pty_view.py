#!/usr/bin/env python3
"""Run any skigard command in a real pty the way a human sees it.

usage: pty_view.py BINARY HOME [--plain] [--answer TEXT] -- ARGS...
With a TERM=xterm-256color pty (colours on) unless --plain (NO_COLOR=1). When the output
contains "anything else to cancel" and --answer is given, the answer is typed.
Prints the exit code, then the transcript; pipe through `cat -v` to see escapes.
The helper detaches from its parent first (see pty_approve.py), so it also works under an agent.
"""
import os
import pty
import select
import sys

argv = sys.argv[1:]
binary, home = argv[0], argv[1]
rest = argv[2:]
plain = "--plain" in rest
answer = None
if "--answer" in rest:
    answer = rest[rest.index("--answer") + 1]
cmd = rest[rest.index("--") + 1:]
AGENT_PREFIXES = ("PI_", "CLAUDE", "CODEX", "GEMINI", "OPENCODE", "CURSOR", "GOOSE", "AMP_")
env = {k: v for k, v in os.environ.items() if not k.startswith(AGENT_PREFIXES)}
env["SKIGARD_HOME"] = home
env["TERM"] = "xterm-256color"
env.pop("NO_COLOR", None)
if plain:
    env["NO_COLOR"] = "1"

r, w = os.pipe()
if os.fork():
    os.close(w)
    with os.fdopen(r, "rb") as f:
        sys.stdout.buffer.write(f.read())
    sys.stdout.flush()
    os._exit(0)
os.setsid()
if os.fork():
    os._exit(0)
os.close(r)
os.dup2(w, 1)

pid, fd = pty.fork()
if pid == 0:
    os.execve(binary, [binary] + cmd, env)
out, sent = b"", False
while True:
    ready, _, _ = select.select([fd], [], [], 10)
    if not ready:
        break
    try:
        data = os.read(fd, 4096)
    except OSError:
        break
    if not data:
        break
    out += data
    if answer is not None and not sent and b"anything else to cancel" in out:
        os.write(fd, answer.encode() + b"\n")
        sent = True
_, status = os.waitpid(pid, 0)
print(os.waitstatus_to_exitcode(status))
print(out.decode(errors="replace").replace("\r\n", "\n"))
sys.stdout.flush()
