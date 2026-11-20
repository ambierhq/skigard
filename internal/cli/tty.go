package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ambierhq/skigard/internal/util"
)

// noTerminal is the refusal shown when approval has no real terminal. It is
// shared by every platform so the message does not drift.
func noTerminal(detail string) error {
	return util.Policyf("this needs a real terminal (%s)", detail)
}

// promptOn shows the summary on the terminal handles the platform file opened
// and requires the human to type want. in and out are always the console
// device (/dev/tty, CONIN$/CONOUT$), never stdin or stdout.
func promptOn(in io.Reader, out io.Writer, summary, want string) error {
	fmt.Fprintln(out, "\n"+summary)
	fmt.Fprintf(out, "\nType %s to confirm, anything else to cancel: ", want)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return util.Cancelled("cancelled")
	}
	if strings.TrimSpace(line) != want {
		return util.Cancelled("cancelled: nothing was approved")
	}
	return nil
}

// ask prompts on stderr and reads one line from stdin.
func ask(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line)
}
