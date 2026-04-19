package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
	errorsAll  bool
	errorsPack string
	errorsTags string
)

var errorsCmd = &cobra.Command{
	Use:   "errors",
	Short: i18n.T("en", "errors.short"),
	Long:  i18n.T("en", "errors.long"),
}

var errorsListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("en", "errors.list.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var packs []*content.Pack

		if errorsPack != "" || errorsAll {
			packs, err = loader.LoadPacks(nil, i18n.ActiveLang)
			if err != nil {
				return err
			}
		} else {
			paths, err := xdg.New()
			if err != nil {
				return err
			}
			profileCfg, err := config.LoadProfile(paths.ConfigDir)
			if err != nil {
				return err
			}
			if profileCfg.ID == "" {
				return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "errors.list.no_profile"))
			}
			activeProfile, err := loader.FindProfile(profileCfg.ID)
			if err != nil {
				return err
			}
			if activeProfile == nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "errors.list.profile_not_found", map[string]any{"ID": profileCfg.ID}))
			}
			packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}

		var errors []content.KnownError
		if errorsPack != "" {
			errors = content.FilterKnownErrorsByPack(packs, errorsPack)
		} else {
			errors = content.FlattenKnownErrors(packs)
		}

		if errorsTags != "" {
			tags := strings.Split(errorsTags, ",")
			errors = content.FilterKnownErrorsByTags(errors, tags)
		}

		if len(errors) == 0 {
			if errorsPack != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "errors.none_pack", map[string]any{"Pack": errorsPack}))
			} else if errorsTags != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "errors.none_tags", map[string]any{"Tags": errorsTags}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "errors.none"))
			}
			return nil
		}
		printErrorTable(errors, true)
		return nil
	},
}

var errorsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T("en", "errors.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		errors := content.FilterKnownErrors(content.FlattenKnownErrors(packs), args[0])
		if len(errors) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "errors.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}
		printErrorDetails(errors)
		return nil
	},
}

func printErrorTable(errors []content.KnownError, showPack bool) {
	colPattern := i18n.T(i18n.ActiveLang, "errors.col_pattern")
	colCause := i18n.T(i18n.ActiveLang, "errors.col_cause")
	colPack := i18n.T(i18n.ActiveLang, "errors.col_pack")
	if showPack {
		fmt.Printf("%-45s %-12s %s\n", colPattern, colPack, colCause)
		fmt.Println(strings.Repeat("-", 100))
		for _, e := range errors {
			fmt.Printf("%-45s %-12s %s\n", truncate(e.Pattern, 44), e.PackID, truncate(e.Cause, 50))
		}
	} else {
		fmt.Printf("%-45s %s\n", colPattern, colCause)
		fmt.Println(strings.Repeat("-", 100))
		for _, e := range errors {
			fmt.Printf("%-45s %s\n", truncate(e.Pattern, 44), truncate(e.Cause, 50))
		}
	}
}

func printErrorDetails(errors []content.KnownError) {
	colPack := i18n.T(i18n.ActiveLang, "errors.col_pack")
	colCause := i18n.T(i18n.ActiveLang, "errors.col_cause")
	colFix := i18n.T(i18n.ActiveLang, "errors.col_fix")
	colDocs := i18n.T(i18n.ActiveLang, "errors.col_docs")
	for i, e := range errors {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("  %s\n", e.Pattern)
		fmt.Printf("  %s:  %s\n", colPack, e.PackID)
		fmt.Printf("  %s: %s\n", colCause, e.Cause)
		fmt.Printf("  %s:   %s\n", colFix, e.Fix)
		if e.Docs != "" {
			fmt.Printf("  %s:  %s\n", colDocs, e.Docs)
		}
	}
}

func init() {
	errorsListCmd.Flags().BoolVarP(&errorsAll, "all", "a", false, "show all errors regardless of profile")
	errorsListCmd.Flags().StringVarP(&errorsPack, "pack", "p", "", "filter to a specific pack")
	errorsListCmd.Flags().StringVarP(&errorsTags, "tags", "t", "", "comma-separated tags (OR match)")
	errorsCmd.AddCommand(errorsListCmd, errorsSearchCmd)
	rootCmd.AddCommand(errorsCmd)
}
