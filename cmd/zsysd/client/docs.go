package client

import (
	//"bytes"
	"golang.org/x/term"
	"os"
	"os/exec"
	"strings"

	//"github.com/alecthomas/chroma/quick"
	"github.com/bbrks/wrap/v2"
	"github.com/spf13/cobra"
	"github.com/ubuntu/zsys/cmd/zsysd/cmdhandler"
	"github.com/ubuntu/zsys/internal/i18n"

	_ "embed"
)

//go:embed docs/whats-new.md
var whatNewContent string

var (
	docsCmd = &cobra.Command{
		Use:   "docs SECTION",
		Short: i18n.G("Embedded Documentation"),
		Args:  cmdhandler.SubcommandsRequiredWithSuggestions,
		Run:   cmdhandler.NoCmd,
	}
	docWhatsNewCmd = &cobra.Command{
		Use:   "whatsnew",
		Short: i18n.G("Whats New"),
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, args []string) { cmdErr = renderDocs("WhatsNew") },
	}
)

func init() {
	rootCmd.AddCommand(docsCmd)
	docsCmd.AddCommand(docWhatsNewCmd)
}

func renderDocs(topic string) error {
	var output = ""
	if topic == "WhatsNew" {
		output = strings.ReplaceAll(whatNewContent, "#cmd#", "cmd")
	}

	width, height, err := term.GetSize(0)
	if err != nil {
		return err
	}

	w := wrap.NewWrapper()

	// Prefix each new line with a single-line comment symbol.
	w.OutputLinePrefix = "   "

	renderedText := w.Wrap(i18n.G(output), width)
	// buf := bytes.NewBufferString("")
	// err = quick.Highlight(buf, renderedText, "markdown", "tty_truecolour", "monokai")
	// renderedText = buf.String()

	if len(renderedText) > (8 * width * height / 10) {
		// Could read $PAGER rather than hardcoding the path.

		pager := os.Getenv("PAGER")
		if "" == pager {
			pager = "/usr/bin/less"
		}
		cmd := exec.Command(pager)

		// Feed it with the string you want to display.
		cmd.Stdin = strings.NewReader(renderedText)

		// This is crucial - otherwise it will write to a null device.
		cmd.Stdout = os.Stdout

		// Fork off a process and wait for it to terminate.
		err = cmd.Run()
	} else {
		_, err = os.Stdout.WriteString(renderedText)
	}
	return err
}
