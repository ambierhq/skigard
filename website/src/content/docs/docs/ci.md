---
title: Using skigard in CI
description: Fail a pull request when installed skills drift from the lockfile, with skigard verify.
---

Commit the lockfile and the approved store, then check them on every pull request:

```yaml
# .github/workflows/skills.yml
- run: skigard verify --strict
```

`verify` exits `1` when the lockfile, the approved store, the installs or the generated marketplace disagree.
Project-scope installs (`skigard install ... --scope project`) copy approved skills into the repository, so the
repository is self-contained. Keep the project lockfile with `SKIGARD_HOME=./.skigard`.

CI can run `scan` too, for example `skigard scan ./skills --format sarif --fail-on high`. CI can never approve:
approval needs a real terminal.
