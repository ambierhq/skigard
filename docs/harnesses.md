# Harness registry

<!-- Generated from internal/harness/registry.toml. Run: go test ./internal/harness -run Docs -update -->

78 harnesses. **Verified** means skigard's authors confirmed the skills directory against the harness's own
documentation or a machine that runs it. Unverified entries are detected and listed, but skigard only installs to
them when you pass `--include-unverified`. Please open a PR to verify or fix an entry.

| ID | Name | Verified | User-scope dir | Project dir | Detected by |
|---|---|---|---|---|---|
| `claude-code` | Claude Code | yes | `~/.claude/skills` (or `$CLAUDE_CONFIG_DIR/skills`) | `.claude/skills` | `claude`, `~/.claude` |
| `omp` | Oh My Pi | yes | `~/.agents/skills` | `.agents/skills` | `omp`, `~/.omp` |
| `pi` | Pi | yes | `~/.agents/skills` | `.agents/skills` | `pi`, `~/.pi` |
| `adal` | AdaL | no | `~/.adal/skills` | `.adal/skills` | `~/.adal` |
| `aider-desk` | AiderDesk | no | `~/.aider-desk/skills` | `.aider-desk/skills` | `~/.aider-desk` |
| `amp` | Amp | no | `{config}/agents/skills` | `.agents/skills` | `amp`, `{config}/amp` |
| `antigravity` | Antigravity | no | `~/.gemini/config/skills` | `.agents/skills` | `/Applications/Antigravity.app`, `~/.gemini/antigravity` |
| `antigravity-cli` | Antigravity CLI | no | `~/.gemini/config/skills` | `.agents/skills` | `agy`, `~/.gemini/antigravity-cli` |
| `astrbot` | AstrBot | no | `~/.astrbot/data/skills` | `data/skills` | `~/.astrbot` |
| `augment` | Augment | no | `~/.augment/skills` | `.augment/skills` | `auggie`, `~/.augment` |
| `autohand-code` | Autohand Code | no | `~/.autohand/skills` (or `$AUTOHAND_HOME/skills`) | `.autohand/skills` | `autohand`, `~/.autohand` |
| `bob` | Bob | no | `~/.bob/skills` | `.bob/skills` | `~/.bob` |
| `cline` | Cline | no | `~/.agents/skills` | `.agents/skills` | `cline`, `~/.cline` |
| `codearts-agent` | CodeArts Agent | no | `~/.codeartsdoer/skills` | `.codeartsdoer/skills` | `~/.codeartsdoer` |
| `codebuddy` | CodeBuddy | no | `~/.codebuddy/skills` | `.codebuddy/skills` | `codebuddy`, `~/.codebuddy` |
| `codemaker` | CodeMaker | no | `~/.codemaker/skills` | `.codemaker/skills` | `~/.codemaker` |
| `codestudio` | CodeStudio | no | `~/.codestudio/skills` | `.codestudio/skills` | `~/.codestudio` |
| `codex` | Codex | no | `~/.agents/skills` (or `$CODEX_HOME/skills`) | `.agents/skills` | `codex`, `~/.codex` |
| `command-code` | Command Code | no | `~/.commandcode/skills` | `.commandcode/skills` | `~/.commandcode` |
| `continue` | Continue | no | `~/.continue/skills` | `.continue/skills` | `cn`, `~/.continue` |
| `cortex` | Cortex Code | no | `~/.snowflake/cortex/skills` | `.cortex/skills` | `~/.snowflake/cortex` |
| `crush` | Crush | no | `{config}/crush/skills` | `.crush/skills` | `crush`, `{config}/crush` |
| `cursor` | Cursor | no | `~/.cursor/skills` | `.agents/skills` | `cursor-agent`, `/Applications/Cursor.app`, `~/.cursor` |
| `deepagents` | Deep Agents | no | `~/.deepagents/agent/skills` | `.agents/skills` | `~/.deepagents` |
| `devin` | Devin | no | `{config}/devin/skills` | `.devin/skills` | `devin`, `{config}/devin` |
| `dexto` | Dexto | no | `~/.agents/skills` | `.agents/skills` | `dexto`, `~/.dexto` |
| `eve` | Eve | no | `` | `agent/skills` |  |
| `firebender` | Firebender | no | `~/.firebender/skills` | `.agents/skills` | `~/.firebender` |
| `forgecode` | ForgeCode | no | `~/.forge/skills` | `.forge/skills` | `forge`, `~/.forge` |
| `gemini-cli` | Gemini CLI | no | `~/.gemini/skills` | `.agents/skills` | `gemini`, `~/.gemini` |
| `github-copilot` | GitHub Copilot | no | `~/.copilot/skills` | `.agents/skills` | `copilot`, `~/.copilot` |
| `goose` | Goose | no | `{config}/goose/skills` | `.goose/skills` | `goose`, `{config}/goose` |
| `grok` | Grok CLI | no | `~/.grok/skills` (or `$GROK_HOME/skills`) | `.grok/skills` | `grok`, `~/.grok` |
| `hermes-agent` | Hermes Agent | no | `~/.hermes/skills` (or `$HERMES_HOME/skills`) | `.hermes/skills` | `hermes`, `~/.hermes` |
| `iflow-cli` | iFlow CLI | no | `~/.iflow/skills` | `.iflow/skills` | `iflow`, `~/.iflow` |
| `inference-sh` | inference.sh | no | `~/.inferencesh/skills` | `.inferencesh/skills` | `~/.inferencesh` |
| `jazz` | Jazz | no | `~/.jazz/skills` | `.jazz/skills` | `~/.jazz` |
| `junie` | Junie | no | `~/.junie/skills` | `.junie/skills` | `junie`, `~/.junie` |
| `kilo` | Kilo Code | no | `~/.kilo/skills` | `.agents/skills` | `kilo`, `~/.kilo` |
| `kimchi` | Kimchi | no | `{config}/kimchi/harness/skills` | `.kimchi/skills` | `{config}/kimchi` |
| `kimi-code-cli` | Kimi Code CLI | no | `~/.agents/skills` | `.agents/skills` | `kimi`, `~/.kimi` |
| `kiro-cli` | Kiro CLI | no | `~/.kiro/skills` | `.kiro/skills` | `kiro-cli`, `/Applications/Kiro.app`, `~/.kiro` |
| `kode` | Kode | no | `~/.kode/skills` | `.kode/skills` | `~/.kode` |
| `lingma` | Lingma | no | `~/.lingma/skills` | `.lingma/skills` | `~/.lingma` |
| `loaf` | Loaf | no | `~/.agents/skills` | `.agents/skills` | `~/.loaf` |
| `mcpjam` | MCPJam | no | `~/.mcpjam/skills` | `.mcpjam/skills` | `~/.mcpjam` |
| `minimax-code` | MiniMax Code | no | `~/.minimax/skills` | `.minimax/skills` | `/Applications/MiniMax Code.app`, `~/.minimax` |
| `mistral-vibe` | Mistral Vibe | no | `~/.vibe/skills` (or `$VIBE_HOME/skills`) | `.vibe/skills` | `vibe`, `~/.vibe` |
| `moxby` | Moxby | no | `~/.moxby/skills` | `.moxby/skills` | `~/.moxby` |
| `mux` | Mux | no | `~/.mux/skills` | `.mux/skills` | `~/.mux` |
| `neovate` | Neovate | no | `~/.neovate/skills` | `.neovate/skills` | `neovate`, `~/.neovate` |
| `ona` | Ona | no | `~/.ona/skills` | `.ona/skills` | `~/.ona` |
| `openclaw` | OpenClaw | no | `~/.openclaw/skills` | `skills` | `openclaw`, `~/.openclaw`, `~/.clawdbot`, `~/.moltbot` |
| `opencode` | OpenCode | no | `{config}/opencode/skills` | `.agents/skills` | `opencode`, `{config}/opencode` |
| `openhands` | OpenHands | no | `~/.openhands/skills` | `.openhands/skills` | `openhands`, `~/.openhands` |
| `pochi` | Pochi | no | `~/.pochi/skills` | `.pochi/skills` | `~/.pochi` |
| `posit-assistant` | Posit Assistant | no | `~/.posit/assistant/skills` | `.posit/assistant/skills` | `~/.posit/assistant`, `~/.positai` |
| `promptscript` | PromptScript | no | `~/.agents/skills` | `.agents/skills` |  |
| `qoder` | Qoder | no | `~/.qoder/skills` | `.qoder/skills` | `~/.qoder` |
| `qoder-cn` | Qoder CN | no | `~/.qoder-cn/skills` | `.qoder/skills` | `~/.qoder-cn` |
| `qwen-code` | Qwen Code | no | `~/.qwen/skills` | `.qwen/skills` | `qwen`, `~/.qwen` |
| `reasonix` | Reasonix | no | `~/.reasonix/skills` (or `$REASONIX_HOME/skills`) | `.reasonix/skills` | `~/.reasonix` |
| `replit` | Replit | no | `{config}/agents/skills` | `.agents/skills` | `{config}/replit` |
| `roo` | Roo Code | no | `~/.roo/skills` | `.roo/skills` | `~/.roo` |
| `rovodev` | Rovo Dev | no | `~/.rovodev/skills` | `.rovodev/skills` | `~/.rovodev` |
| `sarvam-code` | Sarvam Code | no | `~/.agents/skills` (or `$SARVAM_HOME/`) | `.agents/skills` | `~/.sarvam` |
| `tabnine-cli` | Tabnine CLI | no | `~/.tabnine/agent/skills` | `.tabnine/agent/skills` | `tabnine`, `~/.tabnine` |
| `terramind` | Terramind | no | `~/.terramind/skills` | `.terramind/skills` | `~/.terramind` |
| `tinycloud` | TinyCloud | no | `~/.tinycloud/skills` | `.tinycloud/skills` | `~/.tinycloud` |
| `trae` | Trae | no | `~/.trae/skills` | `.trae/skills` | `/Applications/Trae.app`, `~/.trae` |
| `trae-cn` | Trae CN | no | `~/.trae-cn/skills` | `.trae/skills` | `/Applications/Trae CN.app`, `~/.trae-cn` |
| `universal` | Universal (.agents) | no | `{config}/agents/skills` | `.agents/skills` | `{config}/agents` |
| `warp` | Warp | no | `~/.agents/skills` | `.agents/skills` | `/Applications/Warp.app`, `~/.warp` |
| `windsurf` | Windsurf | no | `~/.codeium/windsurf/skills` | `.windsurf/skills` | `windsurf`, `/Applications/Windsurf.app`, `~/.codeium/windsurf` |
| `zcode` | ZCode | no | `~/.zcode/skills` | `.zcode/skills` | `/Applications/ZCode.app`, `~/.zcode` |
| `zed` | Zed | no | `~/.agents/skills` | `.agents/skills` | `zed`, `/Applications/Zed.app`, `{config}/zed` |
| `zencoder` | Zencoder | no | `~/.zencoder/skills` | `.zencoder/skills` | `~/.zencoder` |
| `zenflow` | Zenflow | no | `~/.zencoder/skills` | `.zencoder/skills` | `~/.zencoder` |

`~` is your home directory and `{config}` is `$XDG_CONFIG_HOME`, or `~/.config` when that is unset.
Harnesses that share a directory (for example `~/.agents/skills`) are installed to once.
