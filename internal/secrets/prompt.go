package secrets

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/gregberns/adze/internal/config"
	"golang.org/x/term"
)

// promptUser prompts the user interactively for a secret value.
// It prints the description and prompt to stderr, reads from stdin.
// If sensitive is true, terminal echo is disabled during input.
func promptUser(entry config.SecretEntry) (string, error) {
	desc := entry.Description
	if desc == "" {
		desc = entry.Name
	}

	fmt.Fprintf(os.Stderr, "%s\n", desc)

	if entry.Sensitive {
		fmt.Fprintf(os.Stderr, "Enter %s: ", entry.Name)
		// Read with echo disabled
		fd := int(os.Stdin.Fd())
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr) // newline after hidden input
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	fmt.Fprintf(os.Stderr, "Enter %s: ", entry.Name)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
