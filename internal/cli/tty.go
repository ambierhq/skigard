package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ambierhq/skigard/internal/util"
)

// ttyConfirm shows the summary on the controlling terminal and requires the
// human to type want. It deliberately has no flag, environment variable or
// stdin fallback: if there is no terminal, there is no approval.
func ttyConfirm(summary, want string) error {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return util.Policyf("approving needs a real terminal (no controlling terminal found)")
	}
	defer tty.Close()
	fmt.Fprintln(tty, "\n"+summary)
	fmt.Fprintf(tty, "\nType the skill name (%s) to approve, anything else to cancel: ", want)
	line, err := bufio.NewReader(tty).ReadString('\n')
	if err != nil && line == "" {
		return util.Cancelled("cancelled")
	}
	if strings.TrimSpace(line) != want {
		return util.Cancelled("cancelled: nothing was approved")
	}
	return nil
}

// stdinIsTerminal reports whether prompts can be answered interactively.
func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// ask prompts on stderr and reads one line from stdin.
func ask(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line)
}
