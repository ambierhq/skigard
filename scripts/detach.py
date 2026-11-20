#!/usr/bin/env python3
"""Run a command re-parented (double fork + setsid) and relay its output.

usage: detach.py CMD [ARG...]

skigard's approve refuses when an agent harness is among its ancestors. The e2e
must pass inside and outside an agent, so steps that need to reach the prompt
logic leave the process tree first. That is the documented evasion in
docs/threat-model.md, not something skigard supports.
"""
import os
import subprocess
import sys

r, w = os.pipe()
if os.fork():
    os.close(w)
    with os.fdopen(r, "rb") as f:
        sys.stdout.buffer.write(f.read())
    sys.exit(0)
os.setsid()
if os.fork():
    os._exit(0)
os.close(r)
p = subprocess.run(sys.argv[1:], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, check=False)
os.write(w, p.stdout)
os._exit(0)
