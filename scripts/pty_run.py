#!/usr/bin/env python3
"""Run any skigard command on a real pseudo-terminal and answer its prompt like a human would.

usage: pty_run.py BINARY HOME ANSWER ARGS...
Prints the exit code on the first line, then the terminal transcript.

A human is not a descendant of an agent, so the helper detaches itself (double fork +
setsid) before driving the pty, like pty_approve.py. That is the re-parenting evasion
documented in docs/threat-model.md; it lets the e2e pass inside an agent.
"""
import os
import pty
import select
import sys

binary, home, answer = sys.argv[1:4]
args = sys.argv[4:]
AGENT_PREFIXES = ("PI_", "CLAUDE", "CODEX", "GEMINI", "OPENCODE", "CURSOR", "GOOSE", "AMP_")
env = {k: v for k, v in os.environ.items() if not k.startswith(AGENT_PREFIXES)}
env["SKIGARD_HOME"] = home


def detach():
    """Run the rest in a re-parented grandchild; relay its report through a pipe."""
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


detach()
pid, fd = pty.fork()
if pid == 0:
    os.execve(binary, [binary, *args], env)
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
