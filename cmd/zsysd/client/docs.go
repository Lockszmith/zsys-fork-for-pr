package client

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ubuntu/zsys/cmd/zsysd/cmdhandler"
	"github.com/ubuntu/zsys/internal/i18n"

	_ "embed"
)

//go:embed whats-new.md
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

	_, err := os.Stdout.WriteString(fmt.Sprintf(i18n.G(output)))
	return err
}
