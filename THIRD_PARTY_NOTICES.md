# Third-party notices

skigard is MIT licensed. This file lists code that ships inside the binary, and projects whose
ideas shaped skigard. **No code was copied from the projects in the second list**; they were studied and the
behaviour was reimplemented.

## Included in the binary

### github.com/BurntSushi/toml (MIT)

    The MIT License (MIT)
    
    Copyright (c) 2013 TOML authors
    
    Permission is hereby granted, free of charge, to any person obtaining a copy
    of this software and associated documentation files (the "Software"), to deal
    in the Software without restriction, including without limitation the rights
    to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
    copies of the Software, and to permit persons to whom the Software is
    furnished to do so, subject to the following conditions:
    
    The above copyright notice and this permission notice shall be included in
    all copies or substantial portions of the Software.
    
    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
    THE SOFTWARE.

## Projects that inspired skigard (ideas only, nothing copied)

Thank you to the authors of these projects. Please look at them too.

- **SkillGuardrail** (T-Zevin, Apache-2.0): quarantine-first fetching, bounded and hostile-archive-safe extraction,
  hash-bound baselines that can never suppress high severity, staged atomic installs, receipts kept outside the
  skill tree, and capability chains such as secret read plus network egress.
  https://github.com/T-Zevin/SkillGuardrail
- **SkillSpector** (NVIDIA, Apache-2.0): stable rule identifiers, SARIF output, "coverage incomplete" as a
  first-class state, hidden-content and dependency-redirection rules, and the capability-stripped way of running a
  local agent CLI as a reviewer (prompt on stdin, tools disabled, scrubbed environment).
  https://github.com/NVIDIA/SkillSpector
- **agent-skill-manager / asm** (luongnv89, MIT): a `doctor` command, `--json` everywhere, context-cost
  ("attention budget") reporting, harness definitions as data, and terminal-escape stripping of untrusted text.
  https://github.com/luongnv89/asm
- **vercel-labs/skills** (Vercel, MIT): the breadth of the agent-harness landscape (about 75 agents), the
  "universal" `.agents/skills` directory, per-harness environment overrides, and a deterministic, versioned content
  hash for a skill folder. skigard's harness table was written independently from the paths each harness documents,
  informed by that survey.
  https://github.com/vercel-labs/skills
- **skillpin** (nbsp1221, MIT): a manifest plus lockfile for skills pinned to exact commits.
  https://github.com/nbsp1221/skillpin
- The **Agent Skills** format (a directory with a `SKILL.md`), which every supported harness reads.

## Website (`website/`)

- Newsreader (The Newsreader Project Authors) and IBM Plex Sans (IBM Corp.) fonts, SIL Open Font License 1.1,
  self-hosted from the `@fontsource` packages. The website's other npm dependencies (Astro, Starlight, Lighthouse CI,
  Wrangler) are build-time only and are pinned in `website/package-lock.json`; none ships in the `skigard` binary.
