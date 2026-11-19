package cli

import (
	"fmt"
	"strings"

	"github.com/ambierhq/skigard/internal/util"
)

// completionShells lists the shells `skigard completion` can generate for.
var completionShells = []string{"bash", "zsh", "fish", "powershell"}

func cmdCompletion(c *ctx, args []string) error {
	fs := c.flags("completion")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return util.Errf("usage: skigard completion %s", strings.Join(completionShells, "|"))
	}
	script, err := completionScript(pos[0])
	if err != nil {
		return err
	}
	c.printf("%s", script)
	return nil
}

// completionScript renders a completion script that completes command names.
// It is generated from the command table and never runs anything itself.
func completionScript(shell string) (string, error) {
	names := make([]string, 0, len(commands)+1)
	for _, cm := range commands {
		names = append(names, cm.name)
	}
	names = append(names, "help")
	words := strings.Join(names, " ")
	switch shell {
	case "bash":
		return fmt.Sprintf(`# bash completion for skigard
_skigard() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=($(compgen -W "%s" -- "$cur"))
  else
    COMPREPLY=($(compgen -f -- "$cur"))
  fi
}
complete -F _skigard skigard
`, words), nil
	case "zsh":
		return fmt.Sprintf(`#compdef skigard
_skigard() {
  if (( CURRENT == 2 )); then
    compadd -- %s
  else
    _files
  fi
}
compdef _skigard skigard
`, words), nil
	case "fish":
		var b strings.Builder
		b.WriteString("# fish completion for skigard\ncomplete -c skigard -f\n")
		for _, cm := range commands {
			fmt.Fprintf(&b, "complete -c skigard -n '__fish_use_subcommand' -a %s -d '%s'\n", cm.name, strings.ReplaceAll(cm.summary, "'", ""))
		}
		return b.String(), nil
	case "powershell":
		quoted := make([]string, len(names))
		for i, n := range names {
			quoted[i] = "'" + n + "'"
		}
		return fmt.Sprintf(`# PowerShell completion for skigard
Register-ArgumentCompleter -Native -CommandName skigard -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  if ($commandAst.CommandElements.Count -gt 2) { return }
  @(%s) | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
  }
}
`, strings.Join(quoted, ", ")), nil
	}
	return "", util.Errf("unknown shell %q (want %s)", shell, strings.Join(completionShells, "|"))
}
