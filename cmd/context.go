package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/scratch"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: i18n.T("en", "context.short"),
	Long:  i18n.T("en", "context.long"),
	RunE:  runContextList,
}

var contextAddCmd = &cobra.Command{
	Use:   "add <note>",
	Short: i18n.T("en", "context.add.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		lang := i18n.ActiveLang
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := scratch.Add(cwd, args[0]); err != nil {
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("%s", i18n.T(lang, "context.add.empty"))
			}
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "context.add.done"))
		return nil
	},
}

var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("en", "context.list.short"),
	RunE:  runContextList,
}

func runContextList(cmd *cobra.Command, args []string) error {
	lang := i18n.ActiveLang
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	notes, err := scratch.Load(cwd)
	if err != nil {
		return err
	}
	if len(notes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "context.list.empty"))
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "context.list.header"))
	for _, note := range notes {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", note)
	}
	return nil
}

var contextClearCmd = &cobra.Command{
	Use:   "clear",
	Short: i18n.T("en", "context.clear.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		lang := i18n.ActiveLang
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if !scratch.HasNotes(cwd) {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "context.clear.empty"))
			return nil
		}
		if err := scratch.Clear(cwd); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "context.clear.done"))
		return nil
	},
}

func init() {
	contextCmd.AddCommand(contextAddCmd)
	contextCmd.AddCommand(contextListCmd)
	contextCmd.AddCommand(contextClearCmd)
	rootCmd.AddCommand(contextCmd)
}
