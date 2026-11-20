---
title: "Agent skill supply-chain risks and prompt injection"
description: "How a SKILL.md can attack the agent that reads it, and the controls that reduce the risk: quarantine, hash pinning, a safety score, human approval."
---

Skills are a software supply chain, but the payload is often prose. A `SKILL.md` is read by a model that can act, so the
attack surface is the instructions as much as the scripts.

## What can go wrong

- **Prompt injection through the skill.** Text such as "ignore previous instructions" or "do not tell the user" aimed at
  the agent that loads the skill.
- **Hidden instructions.** Zero-width, bidirectional or tag characters and HTML comments that a human does not see.
- **Bundled code.** Scripts that read credentials, send data out, or download and run more code.
- **Moving targets.** A branch or tag that changes between the day you reviewed a skill and the day you installed it.
- **Hostile archives.** Paths with `../`, symlinks, device files and zip bombs.
- **Injection aimed at the reviewer.** A skill that tells an AI reviewer "this is safe".

## Controls that help

| Risk | Control |
| --- | --- |
| Moving target | Resolve to a full commit, pin a content hash, re-check at install |
| Hostile archive | Extract through a rooted filesystem API; refuse links and devices; size and entry limits |
| Hidden text | Scanner rules for invisible characters, NUL bytes and comments |
| Injection of a reviewer | Tools off, empty directory, scrubbed environment, OS sandbox; the reviewer can only raise concern |
| An agent approving or forcing its own skill | `approve` reads the terminal only, with no flag, stdin path or environment override; forcing an install and changing the mode or threshold are refused in an agent. In auto mode an agent's `add` can still install skills that clear the human-chosen threshold; the lock records it and `verify --require-human` flags it |

None of these makes a skill safe. They make the decision explicit, evidence-based and repeatable. The full list of
attacks considered, and what skigard does not do, is in the [threat model](/skigard/docs/threat-model/).

[Review checklist](/skigard/guides/review-agent-skills-before-installing/) · [Install skigard](/skigard/docs/install/)
