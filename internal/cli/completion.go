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
	// subs maps each command group to its verbs, from the groups table.
	var gnames []string
	for _, cm := range commands {
		if len(groups[cm.name]) > 0 {
			gnames = append(gnames, cm.name)
		}
	}
	subWords := func(g string) string {
		var v []string
		for _, s := range groups[g] {
			v = append(v, s.name)
		}
		return strings.Join(v, " ")
	}
	var bashSubs, zshSubs strings.Builder
	for _, g := range gnames {
		fmt.Fprintf(&bashSubs, "\t\t\t%s) COMPREPLY=($(compgen -W \"%s\" -- \"$cur\")) ;;\n", g, subWords(g))
		fmt.Fprintf(&zshSubs, "\t\t\t%s) compadd -- %s ;;\n", g, subWords(g))
	}
	switch shell {
	case "bash":
		return fmt.Sprintf(`# bash completion for skigard
_skigard() {
	local cur="${COMP_WORDS[COMP_CWORD]}"
	if [ "$COMP_CWORD" -eq 1 ]; then
		COMPREPLY=($(compgen -W "%s" -- "$cur"))
	elif [ "$COMP_CWORD" -eq 2 ]; then
		case "${COMP_WORDS[1]}" in
%s			*) COMPREPLY=($(compgen -f -- "$cur")) ;;
		esac
	else
		COMPREPLY=($(compgen -f -- "$cur"))
	fi
}
complete -F _skigard skigard
`, words, bashSubs.String()), nil
	case "zsh":
		return fmt.Sprintf(`#compdef skigard
_skigard() {
	if (( CURRENT == 2 )); then
		compadd -- %s
	elif (( CURRENT == 3 )); then
		case "${words[2]}" in
%s			*) _files ;;
		esac
	else
		_files
	fi
}
compdef _skigard skigard
`, words, zshSubs.String()), nil
	case "fish":
		var b strings.Builder
		b.WriteString("# fish completion for skigard\ncomplete -c skigard -f\n")
		for _, cm := range commands {
			fmt.Fprintf(&b, "complete -c skigard -n '__fish_use_subcommand' -a %s -d '%s'\n", cm.name, strings.ReplaceAll(cm.summary, "'", ""))
		}
		for _, g := range gnames {
			for _, s := range groups[g] {
				fmt.Fprintf(&b, "complete -c skigard -n '__fish_seen_subcommand_from %s; and not __fish_seen_subcommand_from %s' -a %s -d '%s'\n", g, subWords(g), s.name, strings.ReplaceAll(s.summary, "'", ""))
			}
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
