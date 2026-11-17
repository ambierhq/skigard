#!/usr/bin/env python3
"""Drive `skigard approve` through a real pseudo-terminal, like a human would.

usage: pty_approve.py BINARY HOME NAME ANSWER
Prints the exit code on the first line, then the terminal transcript.
"""
import os
import pty
import select
import sys

binary, home, name, answer = sys.argv[1:5]
# Remove every agent marker so approve does not (correctly) refuse.
env = {k: v for k, v in os.environ.items()
       if not k.startswith(("PI_", "CLAUDE", "CODEX", "GEMINI", "OPENCODE", "CURSOR", "GOOSE", "AMP_"))}
env["SKIGARD_HOME"] = home
pid, fd = pty.fork()
if pid == 0:
    os.execve(binary, [binary, "approve", name], env)
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
    if not sent and b"anything else to cancel" in out:
        os.write(fd, answer.encode() + b"\n")
        sent = True
_, status = os.waitpid(pid, 0)
print(os.waitstatus_to_exitcode(status))
print(out.decode(errors="replace"))
