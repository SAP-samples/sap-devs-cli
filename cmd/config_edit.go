package cmd

import (
	"fmt"
	"strconv"
	"time"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit configuration interactively",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}

		applyDefaults(cfg)

		lang := cfg.Language
		location := cfg.Location
		companyRepo := cfg.CompanyRepo
		experienceLevel := cfg.ExperienceLevel
		tipRotation := cfg.Tip.Rotation
		tutorialInteractive := cfg.Tutorial.Interactive
		syncDisabled := cfg.Sync.Disabled

		evLocalRadius := strconv.Itoa(cfg.Events.EffectiveLocalRadius())
		evRegionalRadius := strconv.Itoa(cfg.Events.EffectiveRegionalRadius())
		evNotifyDays := strconv.Itoa(cfg.Events.EffectiveNotifyDays())
		evNotifyMethod := cfg.Events.NotifyMethod

		syncTips := cfg.Sync.Tips.String()
		syncTools := cfg.Sync.Tools.String()
		syncAdvocates := cfg.Sync.Advocates.String()
		syncResources := cfg.Sync.Resources.String()
		syncContext := cfg.Sync.Context.String()
		syncMCP := cfg.Sync.MCP.String()
		syncEvents := cfg.Sync.Events.String()
		syncYouTube := cfg.Sync.YouTube.String()
		syncDiscovery := cfg.Sync.Discovery.String()
		syncTutorials := cfg.Sync.Tutorials.String()
		syncLearning := cfg.Sync.Learning.String()

		generalGroup := huh.NewGroup(
			huh.NewInput().Title("Language").Description("e.g. en, de — empty for auto-detect").Value(&lang),
			huh.NewInput().Title("Location").Description("City, Country — or use 'config location --detect'").Value(&location),
			huh.NewSelect[string]().Title("Experience Level").
				Options(
					huh.NewOption("(not set)", ""),
					huh.NewOption("Beginner", "beginner"),
					huh.NewOption("Intermediate", "intermediate"),
					huh.NewOption("Advanced", "advanced"),
				).Value(&experienceLevel),
			huh.NewInput().Title("Company Repo").Description("Git URL for company content layer").Value(&companyRepo),
		).Title("General")

		prefsGroup := huh.NewGroup(
			huh.NewSelect[string]().Title("Tip Rotation").
				Options(
					huh.NewOption("Daily", "daily"),
					huh.NewOption("Hourly", "hourly"),
					huh.NewOption("Session", "session"),
				).Value(&tipRotation),
			huh.NewConfirm().Title("Interactive Tutorials").
				Description("Open tutorials in step-by-step TUI mode by default").
				Value(&tutorialInteractive),
		).Title("Preferences")

		eventsGroup := huh.NewGroup(
			huh.NewInput().Title("Local Radius (km)").
				Description("Radius for nearby events").
				Value(&evLocalRadius).
				Validate(positiveInt),
			huh.NewInput().Title("Regional Radius (km)").
				Description("Radius for regional events").
				Value(&evRegionalRadius).
				Validate(positiveInt),
			huh.NewInput().Title("Notify Days").
				Description("Lookahead window for event notifications").
				Value(&evNotifyDays).
				Validate(positiveInt),
			huh.NewSelect[string]().Title("Notify Method").
				Options(
					huh.NewOption("Hook", "hook"),
					huh.NewOption("OS notification", "os"),
					huh.NewOption("Both", "both"),
				).Value(&evNotifyMethod),
		).Title("Events")

		syncGroup := huh.NewGroup(
			huh.NewConfirm().Title("Sync Disabled").
				Description("Disable all background content sync").
				Value(&syncDisabled),
			huh.NewInput().Title("Tips TTL").Value(&syncTips).Validate(validDuration),
			huh.NewInput().Title("Tools TTL").Value(&syncTools).Validate(validDuration),
			huh.NewInput().Title("Advocates TTL").Value(&syncAdvocates).Validate(validDuration),
			huh.NewInput().Title("Resources TTL").Value(&syncResources).Validate(validDuration),
			huh.NewInput().Title("Context TTL").Value(&syncContext).Validate(validDuration),
			huh.NewInput().Title("MCP TTL").Value(&syncMCP).Validate(validDuration),
			huh.NewInput().Title("Events TTL").Value(&syncEvents).Validate(validDuration),
			huh.NewInput().Title("YouTube TTL").Value(&syncYouTube).Validate(validDuration),
			huh.NewInput().Title("Discovery TTL").Value(&syncDiscovery).Validate(validDuration),
			huh.NewInput().Title("Tutorials TTL").Value(&syncTutorials).Validate(validDuration),
			huh.NewInput().Title("Learning TTL").Value(&syncLearning).Validate(validDuration),
		).Title("Sync TTLs")

		form := huh.NewForm(generalGroup, prefsGroup, eventsGroup, syncGroup).
			WithTheme(huh.ThemeFunc(huh.ThemeDracula))

		if err := form.Run(); err != nil {
			if err == huh.ErrUserAborted {
				fmt.Fprintln(cmd.OutOrStdout(), "Edit cancelled.")
				return nil
			}
			return err
		}

		cfg.Language = lang
		cfg.Location = location
		cfg.CompanyRepo = companyRepo
		cfg.ExperienceLevel = experienceLevel
		cfg.Tip.Rotation = tipRotation
		cfg.Tutorial.Interactive = tutorialInteractive
		cfg.Sync.Disabled = syncDisabled

		cfg.Events.LocalRadius, _ = strconv.Atoi(evLocalRadius)
		cfg.Events.RegionalRadius, _ = strconv.Atoi(evRegionalRadius)
		cfg.Events.NotifyDays, _ = strconv.Atoi(evNotifyDays)
		cfg.Events.NotifyMethod = evNotifyMethod

		cfg.Sync.Tips, _ = time.ParseDuration(syncTips)
		cfg.Sync.Tools, _ = time.ParseDuration(syncTools)
		cfg.Sync.Advocates, _ = time.ParseDuration(syncAdvocates)
		cfg.Sync.Resources, _ = time.ParseDuration(syncResources)
		cfg.Sync.Context, _ = time.ParseDuration(syncContext)
		cfg.Sync.MCP, _ = time.ParseDuration(syncMCP)
		cfg.Sync.Events, _ = time.ParseDuration(syncEvents)
		cfg.Sync.YouTube, _ = time.ParseDuration(syncYouTube)
		cfg.Sync.Discovery, _ = time.ParseDuration(syncDiscovery)
		cfg.Sync.Tutorials, _ = time.ParseDuration(syncTutorials)
		cfg.Sync.Learning, _ = time.ParseDuration(syncLearning)

		if err := cfg.Save(paths.ConfigDir); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Configuration saved.")
		return nil
	},
}

func applyDefaults(cfg *config.Config) {
	if cfg.Tip.Rotation == "" {
		cfg.Tip.Rotation = "daily"
	}
	if cfg.Events.NotifyMethod == "" {
		cfg.Events.NotifyMethod = "hook"
	}
}

func positiveInt(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if n <= 0 {
		return fmt.Errorf("must be greater than 0")
	}
	return nil
}

func validDuration(s string) error {
	if s == "" {
		return fmt.Errorf("duration required (e.g. 24h, 168h)")
	}
	_, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration — use Go format like 4h, 24h, 168h")
	}
	return nil
}

func init() {
	configCmd.AddCommand(configEditCmd)
}
