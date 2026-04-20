package cmd

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var (
	samplesAll  bool
	samplesPack string
	samplesTags string
)

var samplesCmd = &cobra.Command{
	Use:   "samples",
	Short: i18n.T("en", "samples.short"),
}

var samplesListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("en", "samples.list.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var packs []*content.Pack

		if samplesPack != "" || samplesAll {
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
				return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "samples.list.no_profile"))
			}
			activeProfile, err := loader.FindProfile(profileCfg.ID)
			if err != nil {
				return err
			}
			if activeProfile == nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "samples.list.profile_not_found", map[string]any{"ID": profileCfg.ID}))
			}
			packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}

		var samples []content.Sample
		if samplesPack != "" {
			samples = content.FilterSamplesByPack(packs, samplesPack)
		} else {
			samples = content.FlattenSamples(packs)
		}

		if samplesTags != "" {
			tags := strings.Split(samplesTags, ",")
			samples = content.FilterSamplesByTags(samples, tags)
		}

		if len(samples) == 0 {
			if samplesPack != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "samples.none_pack", map[string]any{"Pack": samplesPack}))
			} else if samplesTags != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "samples.none_tags", map[string]any{"Tags": samplesTags}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "samples.none"))
			}
			return nil
		}
		printSampleTable(samples, false)
		return nil
	},
}

var samplesSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T("en", "samples.search.short"),
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
		samples := content.FilterSamples(content.FlattenSamples(packs), args[0])
		if len(samples) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "samples.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}
		printSampleTable(samples, true)
		return nil
	},
}

var samplesOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: i18n.T("en", "samples.open.short"),
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
		s := content.FindSample(content.FlattenSamples(packs), args[0])
		if s == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "samples.not_found", map[string]any{"ID": args[0]}))
		}
		if err := browser.OpenURL(s.URL); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "samples.open.browser_fail", map[string]any{"Err": err, "URL": s.URL}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "samples.open.opening", map[string]any{"Label": s.Label, "URL": s.URL}))
		return nil
	},
}

var samplesCloneCmd = &cobra.Command{
	Use:   "clone <id>",
	Short: i18n.T("en", "samples.clone.short"),
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
		s := content.FindSample(content.FlattenSamples(packs), args[0])
		if s == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "samples.not_found", map[string]any{"ID": args[0]}))
		}
		repoURL, err := repoURLFromGitHub(s.URL)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "samples.clone.not_github", map[string]any{"ID": s.ID}))
		}
		dirName := repoNameFromURL(repoURL)
		if _, err := os.Stat(dirName); err == nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "samples.clone.exists", map[string]any{"Dir": dirName}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "samples.clone.cloning", map[string]any{"Repo": repoURL}))
		gitCmd := exec.Command("git", "clone", repoURL)
		gitCmd.Stdout = cmd.OutOrStdout()
		gitCmd.Stderr = cmd.ErrOrStderr()
		if err := gitCmd.Run(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "samples.clone.done", map[string]any{"Dir": dirName}))
		return nil
	},
}

func repoURLFromGitHub(fileURL string) (string, error) {
	u, err := url.Parse(fileURL)
	if err != nil {
		return "", err
	}
	if u.Host != "github.com" {
		return "", fmt.Errorf("not a GitHub URL")
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("not a GitHub URL")
	}
	return fmt.Sprintf("https://github.com/%s/%s", parts[0], parts[1]), nil
}

func repoNameFromURL(repoURL string) string {
	parts := strings.Split(strings.TrimRight(repoURL, "/"), "/")
	return parts[len(parts)-1]
}

func printSampleTable(samples []content.Sample, showPack bool) {
	colID := i18n.T(i18n.ActiveLang, "samples.col_id")
	colPack := i18n.T(i18n.ActiveLang, "samples.col_pack")
	colLabel := i18n.T(i18n.ActiveLang, "samples.col_label")
	colTags := i18n.T(i18n.ActiveLang, "samples.col_tags")
	if showPack {
		fmt.Printf("%-35s %-12s %-35s %s\n", colID, colPack, colLabel, colTags)
		fmt.Println(strings.Repeat("-", 95))
		for _, s := range samples {
			fmt.Printf("%-35s %-12s %-35s %s\n", s.ID, s.PackID, s.Label, strings.Join(s.Tags, ","))
		}
	} else {
		fmt.Printf("%-35s %-35s %s\n", colID, colLabel, colTags)
		fmt.Println(strings.Repeat("-", 80))
		for _, s := range samples {
			fmt.Printf("%-35s %-35s %s\n", s.ID, s.Label, strings.Join(s.Tags, ","))
		}
	}
}

func init() {
	samplesListCmd.Flags().BoolVarP(&samplesAll, "all", "a", false, "show all samples regardless of profile")
	samplesListCmd.Flags().StringVarP(&samplesPack, "pack", "p", "", "filter to a specific pack")
	samplesListCmd.Flags().StringVarP(&samplesTags, "tags", "t", "", "comma-separated tags (OR match)")
	samplesCmd.AddCommand(samplesListCmd, samplesSearchCmd, samplesOpenCmd, samplesCloneCmd)
	rootCmd.AddCommand(samplesCmd)
}
