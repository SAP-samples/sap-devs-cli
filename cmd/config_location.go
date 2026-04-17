package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var locationDetectFlag bool

var configLocationCmd = &cobra.Command{
	Use:   "location [value]",
	Short: i18n.T(i18n.ActiveLang, "config.location.short"),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if locationDetectFlag && len(args) > 0 {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "config.location.detect_with_value"))
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}

		if locationDetectFlag {
			loc, err := detectLocation(cmd.OutOrStdout(), os.Stdin)
			if err != nil {
				return err
			}
			if loc == "" {
				return nil
			}
			cfg.Location = loc
			if err := cfg.Save(paths.ConfigDir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.location.done", map[string]any{"Value": loc}))
			return nil
		}

		if len(args) == 1 {
			cfg.Location = args[0]
			if err := cfg.Save(paths.ConfigDir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.location.done", map[string]any{"Value": args[0]}))
			return nil
		}

		// No args, no flag: show current value
		val := cfg.Location
		if val == "" {
			val = i18n.T(i18n.ActiveLang, "config.location.not_set")
		}
		fmt.Fprintln(cmd.OutOrStdout(), val)
		return nil
	},
}

// detectLocation fetches approximate location from ip-api.com.
// Prints the privacy notice and confirm prompt to w; reads the confirmation line from r.
// Returns the location string if confirmed, or ("", nil) if declined or on HTTP failure.
func detectLocation(w io.Writer, r io.Reader) (string, error) {
	fmt.Fprintln(w, i18n.T(i18n.ActiveLang, "config.location.detect_notice"))

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://ip-api.com/json")
	if err != nil {
		fmt.Fprintln(w, i18n.Tf(i18n.ActiveLang, "config.location.detect_failed", map[string]any{"Err": err.Error()}))
		return "", nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintln(w, i18n.Tf(i18n.ActiveLang, "config.location.detect_failed", map[string]any{"Err": "HTTP error"}))
		return "", nil
	}

	var result struct {
		City    string `json:"city"`
		Country string `json:"country"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.City == "" {
		fmt.Fprintln(w, i18n.Tf(i18n.ActiveLang, "config.location.detect_failed", map[string]any{"Err": "could not parse response"}))
		return "", nil
	}

	detected := result.City + ", " + result.Country
	fmt.Fprint(w, i18n.Tf(i18n.ActiveLang, "config.location.detect_confirm", map[string]any{"Value": detected}))

	scanner := bufio.NewScanner(r)
	scanner.Scan()
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if answer == "n" || answer == "no" {
		fmt.Fprintln(w, i18n.T(i18n.ActiveLang, "config.location.detect_cancelled"))
		return "", nil
	}
	return detected, nil
}

func init() {
	configLocationCmd.Flags().BoolVar(&locationDetectFlag, "detect", false, "Auto-detect location from IP address")
}
