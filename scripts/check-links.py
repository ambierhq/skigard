#!/usr/bin/env python3
"""Check relative links and #anchors in the repository's Markdown files (no network).

Run from anywhere: python3 scripts/check-links.py
External URLs are checked by the weekly lychee workflow (.github/workflows/links.yml).
"""
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SKIP_DIRS = {".git", "dist", "node_modules", "testdata", "maintenance"}
LINK = re.compile(r"(?<!\!)\[[^\]]*\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")
HTML = re.compile(r"""(?:href|src|srcset)=["']([^"']+)["']""")


def slug(heading):
    s = re.sub(r"`", "", heading.strip().lower())
    s = re.sub(r"[^\w\- ]", "", s)
    return s.replace(" ", "-")


def anchors(path, cache={}):
    if path not in cache:
        found, fence = set(), False
        with open(path, encoding="utf-8") as f:
            for line in f:
                if line.startswith("```"):
                    fence = not fence
                m = None if fence else re.match(r"#{1,6}\s+(.*)", line)
                if m:
                    found.add(slug(m.group(1)))
        cache[path] = found
    return cache[path]


def files():
    for d, dirs, names in os.walk(ROOT):
        dirs[:] = [x for x in dirs if x not in SKIP_DIRS]
        for n in names:
            if n.endswith(".md"):
                yield os.path.join(d, n)


def main():
    bad = 0
    for path in files():
        text = open(path, encoding="utf-8").read()
        text = re.sub(r"<!--.*?-->", "", text, flags=re.S)  # commented-out blocks
        text = re.sub(r"```.*?```", "", text, flags=re.S)
        for target in LINK.findall(text) + HTML.findall(text):
            if re.match(r"^(https?:|mailto:|data:)", target):
                continue
            if target.startswith("/skigard/"):
                continue  # site-absolute URL (website/src/content/docs/guides); website `npm run check:links` verifies them on the built site
            ref, _, frag = target.partition("#")
            dest = path if ref == "" else os.path.normpath(os.path.join(os.path.dirname(path), ref))
            rel = os.path.relpath(path, ROOT)
            if not os.path.exists(dest):
                print(f"{rel}: missing file {target}")
                bad += 1
            elif frag and dest.endswith(".md") and frag not in anchors(dest):
                print(f"{rel}: missing anchor {target}")
                bad += 1
    print("links ok" if not bad else f"{bad} broken link(s)")
    return 1 if bad else 0


sys.exit(main())
