package cli

import (
	"strings"

	"github.com/ambierhq/skigard/internal/term"
	"github.com/ambierhq/skigard/internal/util"
)

func cmdAllow(c *ctx, args []string) error {
	sub, args, err := c.verb("allow", args)
	if err != nil {
		return err
	}
	fs := c.flags("allow " + sub)
	reason := fs.String("reason", "", "why these rules are fine for this source (required with add)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	st := c.style(false)
	switch sub {
	case "add":
		if len(pos) < 2 {
			return util.Errf("usage: skigard allow add SOURCE|SKILL RULE... --reason R")
		}
		src, err := c.app.ResolveAllowSource(pos[0])
		if err != nil {
			return err
		}
		if err := c.app.AllowAdd(src, pos[1:], *reason, ttyConfirm); err != nil {
			return err
		}
		c.printf("%s allowed %s for %s\n%s skigard scan NAME   (suppressed findings are counted, not shown)\n", st.Green("✔"), strings.Join(pos[1:], ", "), util.Clean(src), st.Dim("next:"))
		return nil
	case "remove":
		if len(pos) < 1 {
			return util.Errf("usage: skigard allow remove SOURCE [RULE...]")
		}
		src, err := c.app.ResolveAllowSource(pos[0])
		if err != nil {
			return err
		}
		if err := c.app.AllowRemove(src, pos[1:], ttyConfirm); err != nil {
			return err
		}
		c.printf("%s removed from the allow list: %s\n", st.Green("✔"), util.Clean(src))
		return nil
	case "list":
		es, err := c.app.AllowList()
		if err != nil {
			return err
		}
		if c.json {
			if es == nil {
				return c.printJSON([]struct{}{})
			}
			return c.printJSON(es)
		}
		if len(es) == 0 {
			c.printf("nothing is allowed. Add: skigard allow add SOURCE RULE... --reason R\n")
			return nil
		}
		var rows [][]string
		for _, e := range es {
			rows = append(rows, []string{util.Clean(e.Source), strings.Join(e.Rules, ", "), util.Clean(e.Reason)})
		}
		c.printf("%s", term.Table(st, []string{"SOURCE", "RULES", "REASON"}, rows, func(_, col int, cell string) string {
			if col == 0 {
				return st.Bold(cell)
			}
			return cell
		}))
		return nil
	}
	return util.Errf("unknown allow command %q", sub) // unreachable: verb() validated it
}
