---
title: Getting started
description: From zero to your first human-approved skill in six commands.
---

```sh
skigard detect                                       # which agent harnesses are on this machine?
skigard add github:tt-a1i/archify --skill archify    # fetch into quarantine and scan
skigard review archify                               # optional second opinion from an agent you already use
skigard approve archify                              # in a real terminal: you type the skill name
skigard install archify                              # link into every detected, verified harness
skigard verify                                       # CI-friendly: lock vs store vs installs
```

`approve` shows the scan, the review and the diff against anything approved before, then asks you to type the
skill name. It reads the terminal directly, so an agent running these commands cannot answer for you.

To update later, run `skigard update`. Anything that changed goes back to quarantine with a diff and needs a new
review and approval. On a new machine, clone your home repository to `~/.config/skigard` and run `skigard sync`.

Next: [Concepts](/skigard/docs/concepts/), [Threat model](/skigard/docs/threat-model/).
