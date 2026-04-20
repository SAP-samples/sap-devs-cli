# Events Command (Phase 2: Extended Features) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add iCal export, session-start hook reminders, OS notifications, and config subcommands to the events command built in Phase 1.

**Architecture:** Three new files in `internal/events/` (ical.go, notify.go) and `internal/notify/` (notify.go for OS dispatch). New subcommands added to existing `cmd/events.go` (export, hook, notify). Config subcommands in a new `cmd/config_events.go` following the `config_tip_rotation.go` pattern. Additional i18n keys for all new functionality.

**Tech Stack:** Go, Cobra, `os/exec` (OS notifications), iCal RFC 5545 format

**Spec:** `docs/superpowers/specs/2026-04-18-events-design.md`

**Depends on:** Phase 1 complete on `feat/events` branch

---

## File Map

| File | Action | Responsibility |
| --- | --- | --- |
| `internal/events/ical.go` | Create | `ExportICS()` — write iCal format to `io.Writer` |
| `internal/events/notify.go` | Create | `CheckUpcoming()`, `FormatHookMessage()` |
| `internal/notify/notify.go` | Create | `Send()`, `Available()` — cross-platform OS notifications |
| `cmd/events.go` | Modify | Add `eventsExportCmd`, `eventsHookCmd`, `eventsNotifyCmd` + flags |
| `cmd/config_events.go` | Create | `configEventsCmd` parent + 4 setting subcommands |
| `cmd/config.go` | Modify | Register `configEventsCmd`, update `configShowCmd` |
| `internal/i18n/catalogs/en.json` | Modify | Add Phase 2 i18n keys |
| `internal/i18n/catalogs/de.json` | Modify | Add Phase 2 German i18n keys |

---

### Task 1: Calendar Export — internal/events/ical.go and calendar.go

**Files:**
- Create: `internal/events/ical.go`
- Create: `internal/events/calendar.go`

- [ ] **Step 1: Create ical.go (iCal and vCalendar export)**

```go
package events

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

// ExportICS writes events in iCal format (RFC 5545) to w.
func ExportICS(evts []content.EventInstance, w io.Writer) error {
	fmt.Fprintln(w, "BEGIN:VCALENDAR")
	fmt.Fprintln(w, "VERSION:2.0")
	fmt.Fprintln(w, "PRODID:-//sap-devs//events//EN")
	fmt.Fprintln(w, "CALSCALE:GREGORIAN")
	fmt.Fprintln(w, "METHOD:PUBLISH")

	for _, e := range evts {
		fmt.Fprintln(w, "BEGIN:VEVENT")
		fmt.Fprintf(w, "UID:%s@sap-devs\n", e.ID)

		if dt, err := e.ParseDate(); err == nil {
			fmt.Fprintf(w, "DTSTART;VALUE=DATE:%s\n", formatICSDate(dt))
		}
		if e.EndDateStr != "" {
			if dt, err := e.ParseEndDate(); err == nil {
				// iCal DTEND for all-day events is exclusive, so add one day
				fmt.Fprintf(w, "DTEND;VALUE=DATE:%s\n", formatICSDate(dt.AddDate(0, 0, 1)))
			}
		}

		fmt.Fprintf(w, "SUMMARY:%s\n", escapeICS(e.Title))
		if e.Location != "" && !strings.EqualFold(e.Location, "virtual") {
			fmt.Fprintf(w, "LOCATION:%s\n", escapeICS(e.Location))
		}
		if e.URL != "" {
			fmt.Fprintf(w, "URL:%s\n", e.URL)
		}

		fmt.Fprintf(w, "DTSTAMP:%s\n", time.Now().UTC().Format("20060102T150405Z"))
		fmt.Fprintln(w, "END:VEVENT")
	}

	fmt.Fprintln(w, "END:VCALENDAR")
	return nil
}

func formatICSDate(t time.Time) string {
	return t.Format("20060102")
}

func escapeICS(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
```

- [ ] **Step 2: Create calendar.go (Google Calendar URL, Outlook URL, vCalendar)**

```go
package events

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

// GoogleCalendarURL returns a URL that opens Google Calendar with the event pre-filled.
func GoogleCalendarURL(e content.EventInstance) string {
	dt, _ := e.ParseDate()
	start := dt.Format("20060102")
	end := start
	if e.EndDateStr != "" {
		if edt, err := e.ParseEndDate(); err == nil {
			end = edt.AddDate(0, 0, 1).Format("20060102")
		}
	} else {
		end = dt.AddDate(0, 0, 1).Format("20060102")
	}

	params := url.Values{}
	params.Set("action", "TEMPLATE")
	params.Set("text", e.Title)
	params.Set("dates", start+"/"+end)
	if e.Location != "" && !strings.EqualFold(e.Location, "virtual") {
		params.Set("location", e.Location)
	}
	if e.URL != "" {
		params.Set("details", "Details: "+e.URL)
	}
	return "https://calendar.google.com/calendar/render?" + params.Encode()
}

// OutlookWebURL returns a URL that opens Outlook Web with the event pre-filled.
func OutlookWebURL(e content.EventInstance) string {
	dt, _ := e.ParseDate()
	start := dt.Format("2006-01-02")
	end := start
	if e.EndDateStr != "" {
		if edt, err := e.ParseEndDate(); err == nil {
			end = edt.AddDate(0, 0, 1).Format("2006-01-02")
		}
	} else {
		end = dt.AddDate(0, 0, 1).Format("2006-01-02")
	}

	params := url.Values{}
	params.Set("path", "/calendar/action/compose")
	params.Set("rru", "addevent")
	params.Set("subject", e.Title)
	params.Set("startdt", start)
	params.Set("enddt", end)
	params.Set("allday", "true")
	if e.Location != "" && !strings.EqualFold(e.Location, "virtual") {
		params.Set("location", e.Location)
	}
	if e.URL != "" {
		params.Set("body", "Details: "+e.URL)
	}
	return "https://outlook.live.com/calendar/0/deeplink/compose?" + params.Encode()
}

// ExportVCS writes events in vCalendar 1.0 format (.vcs) to w.
func ExportVCS(evts []content.EventInstance, w io.Writer) error {
	fmt.Fprintln(w, "BEGIN:VCALENDAR")
	fmt.Fprintln(w, "VERSION:1.0")
	fmt.Fprintln(w, "PRODID:-//sap-devs//events//EN")

	for _, e := range evts {
		fmt.Fprintln(w, "BEGIN:VEVENT")
		if dt, err := e.ParseDate(); err == nil {
			fmt.Fprintf(w, "DTSTART:%s\n", dt.Format("20060102"))
		}
		if e.EndDateStr != "" {
			if dt, err := e.ParseEndDate(); err == nil {
				fmt.Fprintf(w, "DTEND:%s\n", dt.AddDate(0, 0, 1).Format("20060102"))
			}
		}
		fmt.Fprintf(w, "SUMMARY:%s\n", e.Title)
		if e.Location != "" && !strings.EqualFold(e.Location, "virtual") {
			fmt.Fprintf(w, "LOCATION:%s\n", e.Location)
		}
		if e.URL != "" {
			fmt.Fprintf(w, "URL:%s\n", e.URL)
		}
		fmt.Fprintln(w, "END:VEVENT")
	}

	fmt.Fprintln(w, "END:VCALENDAR")
	return nil
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/events/ical.go internal/events/calendar.go
git commit -m "feat(events): add calendar export (iCal, vCalendar, Google Calendar URL, Outlook URL)"
```

---

### Task 2: Notification Logic — internal/events/notify.go

**Files:**
- Create: `internal/events/notify.go`

- [ ] **Step 1: Create notify.go**

```go
package events

import (
	"fmt"
	"strings"
	"time"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

// CheckUpcoming returns events whose date is within the next N days.
func CheckUpcoming(evts []content.EventInstance, withinDays int) []content.EventInstance {
	now := time.Now()
	cutoff := now.AddDate(0, 0, withinDays)
	var upcoming []content.EventInstance
	for _, e := range evts {
		dt, err := e.ParseDate()
		if err != nil {
			continue
		}
		if (dt.Equal(now) || dt.After(now)) && dt.Before(cutoff) {
			upcoming = append(upcoming, e)
		}
	}
	return upcoming
}

// FormatHookMessage formats a terminal-friendly reminder for upcoming events.
func FormatHookMessage(upcoming []content.EventInstance) string {
	if len(upcoming) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "📅 %d upcoming SAP event(s):\n", len(upcoming))
	for _, e := range upcoming {
		dt, _ := e.ParseDate()
		date := dt.Format("Jan 2")
		scope := e.Scope
		if scope == "" {
			scope = "event"
		}
		fmt.Fprintf(&b, "  • %s — %s (%s)\n", date, e.Title, scope)
	}
	b.WriteString("Run 'sap-devs events' for details or 'sap-devs events open <id>' to register.")
	return b.String()
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/events/notify.go
git commit -m "feat(events): add upcoming event detection and hook message formatting"
```

---

### Task 3: OS Notifications — internal/notify/notify.go

**Files:**
- Create: `internal/notify/notify.go`

- [ ] **Step 1: Create notify.go**

```go
package notify

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Send dispatches an OS-level notification. Returns an error if unavailable.
func Send(title, body string) error {
	switch runtime.GOOS {
	case "darwin":
		return sendDarwin(title, body)
	case "linux":
		return sendLinux(title, body)
	case "windows":
		return sendWindows(title, body)
	default:
		return fmt.Errorf("OS notifications not supported on %s", runtime.GOOS)
	}
}

// Available reports whether the current platform supports OS notifications.
func Available() bool {
	switch runtime.GOOS {
	case "darwin":
		return true
	case "linux":
		_, err := exec.LookPath("notify-send")
		return err == nil
	case "windows":
		return true
	default:
		return false
	}
}

func sendDarwin(title, body string) error {
	script := fmt.Sprintf(`display notification %q with title %q`, body, title)
	return exec.Command("osascript", "-e", script).Run()
}

func sendLinux(title, body string) error {
	return exec.Command("notify-send", title, body).Run() //nolint:gosec
}

func sendWindows(title, body string) error {
	// Use PowerShell with built-in Windows 10+ toast API
	ps := fmt.Sprintf(
		`[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null; `+
			`$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02); `+
			`$textNodes = $template.GetElementsByTagName('text'); `+
			`$textNodes.Item(0).AppendChild($template.CreateTextNode('%s')) | Out-Null; `+
			`$textNodes.Item(1).AppendChild($template.CreateTextNode('%s')) | Out-Null; `+
			`$toast = [Windows.UI.Notifications.ToastNotification]::new($template); `+
			`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('sap-devs').Show($toast)`,
		escapePS(title), escapePS(body),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err := cmd.Run(); err != nil {
		return sendWindowsFallback(title, body)
	}
	return nil
}

func sendWindowsFallback(title, body string) error {
	msg := fmt.Sprintf("%s\n\n%s", title, body)
	return exec.Command("msg", "*", msg).Run() //nolint:gosec
}

func escapePS(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	s = strings.ReplaceAll(s, "`", "``")
	return s
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/notify/notify.go
git commit -m "feat(events): add cross-platform OS notification dispatch"
```

---

### Task 4: i18n Keys for Phase 2

**Files:**
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json`

- [ ] **Step 1: Add English i18n keys**

Add after the existing `events.types.col_name` entry (before `version.*`):

```json
"events.export.short": "Export events to a calendar file or generate calendar URLs",
"events.export.done": "Exported {{.Count}} events to {{.Path}}",
"events.export.none": "No events to export.",
"events.hook.short": "Print upcoming event reminders (session-start hook)",
"events.notify.short": "Send OS notification for upcoming events",
"events.notify.sent": "Notification sent: {{.Count}} upcoming events",
"events.notify.none": "No upcoming events within {{.Days}} days.",
"events.notify.unsupported": "OS notifications not available on this system. Showing reminder instead:",
"config.events.short": "Configure event filtering and notification settings",
"config.events.show": "events.local_radius:    {{.LocalRadius}} km\nevents.regional_radius: {{.RegionalRadius}} km\nevents.notify_days:     {{.NotifyDays}}\nevents.notify_method:   {{.NotifyMethod}}",
"config.events.local_radius.done": "Local radius set to {{.Value}} km",
"config.events.local_radius.current": "events.local_radius: {{.Value}} km",
"config.events.regional_radius.done": "Regional radius set to {{.Value}} km",
"config.events.regional_radius.current": "events.regional_radius: {{.Value}} km",
"config.events.notify_days.done": "Notify days set to {{.Value}}",
"config.events.notify_days.current": "events.notify_days: {{.Value}}",
"config.events.notify_method.done": "Notify method set to {{.Value}}",
"config.events.notify_method.current": "events.notify_method: {{.Value}}",
"config.events.notify_method.invalid": "Invalid notify method \"{{.Value}}\": must be hook, os, or both",
"config.events.invalid_number": "Invalid number: {{.Value}}",
```

- [ ] **Step 2: Add German i18n keys**

```json
"events.export.short": "Veranstaltungen als iCal-Datei (.ics) exportieren",
"events.export.done": "{{.Count}} Veranstaltungen nach {{.Path}} exportiert",
"events.export.none": "Keine Veranstaltungen zum Exportieren.",
"events.hook.short": "Erinnerungen für kommende Veranstaltungen ausgeben (Session-Start-Hook)",
"events.notify.short": "OS-Benachrichtigung für kommende Veranstaltungen senden",
"events.notify.sent": "Benachrichtigung gesendet: {{.Count}} kommende Veranstaltungen",
"events.notify.none": "Keine kommenden Veranstaltungen innerhalb von {{.Days}} Tagen.",
"events.notify.unsupported": "OS-Benachrichtigungen auf diesem System nicht verfügbar. Erinnerung stattdessen:",
"config.events.short": "Veranstaltungsfilter- und Benachrichtigungseinstellungen konfigurieren",
"config.events.show": "events.local_radius:    {{.LocalRadius}} km\nevents.regional_radius: {{.RegionalRadius}} km\nevents.notify_days:     {{.NotifyDays}}\nevents.notify_method:   {{.NotifyMethod}}",
"config.events.local_radius.done": "Lokaler Radius auf {{.Value}} km gesetzt",
"config.events.local_radius.current": "events.local_radius: {{.Value}} km",
"config.events.regional_radius.done": "Regionaler Radius auf {{.Value}} km gesetzt",
"config.events.regional_radius.current": "events.regional_radius: {{.Value}} km",
"config.events.notify_days.done": "Benachrichtigungstage auf {{.Value}} gesetzt",
"config.events.notify_days.current": "events.notify_days: {{.Value}}",
"config.events.notify_method.done": "Benachrichtigungsmethode auf {{.Value}} gesetzt",
"config.events.notify_method.current": "events.notify_method: {{.Value}}",
"config.events.notify_method.invalid": "Ungültige Benachrichtigungsmethode \"{{.Value}}\": muss hook, os oder both sein",
"config.events.invalid_number": "Ungültige Zahl: {{.Value}}",
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json
git commit -m "feat(events): add Phase 2 i18n keys (export, notify, config)"
```

---

### Task 5: Event Subcommands — export, hook, notify

**Files:**
- Modify: `cmd/events.go`

- [ ] **Step 1: Add export, hook, and notify subcommands**

Add these variables and commands after `eventsTypesCmd` and before `printEventTable`:

```go
var (
	eventsExportType   string
	eventsExportOutput string
	eventsExportFormat string
)

var eventsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export events to a calendar file or generate calendar URLs",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}

		eventTypes := content.FlattenEventTypes(packs)
		var allEvents []content.EventInstance
		for _, et := range eventTypes {
			if et.Source == "rss" {
				resolved, _ := events.Resolve(et, paths.CacheDir, false)
				allEvents = append(allEvents, resolved...)
			}
		}
		manual := content.FlattenEventInstances(packs)
		allEvents = events.MergeAndSort(allEvents, manual)

		if eventsExportType != "" {
			allEvents = content.FilterEventsByType(allEvents, eventsExportType)
		}

		if !eventsAll && cfg.Location != "" {
			userLat, userLon, ok := geo.Lookup(cfg.Location)
			if ok {
				allEvents = events.FilterByLocation(allEvents, userLat, userLon,
					cfg.Events.EffectiveLocalRadius(), cfg.Events.EffectiveRegionalRadius())
			}
		}

		if len(allEvents) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "events.export.none"))
			return nil
		}

		format := eventsExportFormat
		if format == "" {
			format = "ics"
		}

		switch format {
		case "google":
			for _, e := range allEvents {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n  %s\n\n", e.Title, events.GoogleCalendarURL(e))
			}
			return nil
		case "outlook":
			for _, e := range allEvents {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n  %s\n\n", e.Title, events.OutlookWebURL(e))
			}
			return nil
		case "ics", "vcs":
			outPath := eventsExportOutput
			if outPath == "" {
				outPath = "sap-devs-events." + format
			}
			f, err := os.Create(outPath)
			if err != nil {
				return err
			}
			defer f.Close()

			if format == "vcs" {
				err = events.ExportVCS(allEvents, f)
			} else {
				err = events.ExportICS(allEvents, f)
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.export.done", map[string]any{"Count": len(allEvents), "Path": outPath}))
			return nil
		default:
			return fmt.Errorf("unknown format %q: must be ics, vcs, google, or outlook", format)
		}
	},
}

var eventsHookCmd = &cobra.Command{
	Use:    "hook",
	Short:  "Print upcoming event reminders (session-start hook)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}

		eventTypes := content.FlattenEventTypes(packs)
		var allEvents []content.EventInstance
		for _, et := range eventTypes {
			if et.Source == "rss" {
				resolved, _ := events.Resolve(et, paths.CacheDir, false)
				allEvents = append(allEvents, resolved...)
			}
		}
		manual := content.FlattenEventInstances(packs)
		allEvents = events.MergeAndSort(allEvents, manual)

		upcoming := events.CheckUpcoming(allEvents, cfg.Events.EffectiveNotifyDays())
		if msg := events.FormatHookMessage(upcoming); msg != "" {
			fmt.Fprintln(cmd.OutOrStdout(), msg)
		}
		return nil
	},
}

var eventsNotifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Send OS notification for upcoming events",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}

		eventTypes := content.FlattenEventTypes(packs)
		var allEvents []content.EventInstance
		for _, et := range eventTypes {
			if et.Source == "rss" {
				resolved, _ := events.Resolve(et, paths.CacheDir, false)
				allEvents = append(allEvents, resolved...)
			}
		}
		manual := content.FlattenEventInstances(packs)
		allEvents = events.MergeAndSort(allEvents, manual)

		days := cfg.Events.EffectiveNotifyDays()
		upcoming := events.CheckUpcoming(allEvents, days)
		if len(upcoming) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.notify.none", map[string]any{"Days": days}))
			return nil
		}

		msg := events.FormatHookMessage(upcoming)

		if !notify.Available() {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "events.notify.unsupported"))
			fmt.Fprintln(cmd.OutOrStdout(), msg)
			return nil
		}

		title := fmt.Sprintf("SAP Events: %d upcoming", len(upcoming))
		if err := notify.Send(title, msg); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), msg)
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.notify.sent", map[string]any{"Count": len(upcoming)}))
		return nil
	},
}
```

- [ ] **Step 2: Add imports**

Add to the import block in `cmd/events.go`:

```go
"os"

"github.com/SAP-samples/sap-devs-cli/internal/notify"
```

- [ ] **Step 3: Update init() to register new subcommands and flags**

Change the `init()` function to add the new commands and flags:

```go
func init() {
	eventsCmd.Flags().BoolVarP(&eventsAll, "all", "a", false, "show all events regardless of location")
	eventsCmd.Flags().StringVarP(&eventsType, "type", "t", "", "filter by event type ID")
	eventsCmd.Flags().IntVarP(&eventsCount, "count", "n", 10, "max events to display")
	eventsExportCmd.Flags().StringVarP(&eventsExportType, "type", "t", "", "filter by event type ID")
	eventsExportCmd.Flags().StringVarP(&eventsExportOutput, "output", "o", "", "output file path (default: sap-devs-events.ics)")
	eventsExportCmd.Flags().StringVarP(&eventsExportFormat, "format", "f", "ics", "export format: ics, vcs, google, outlook")
	eventsExportCmd.Flags().BoolVarP(&eventsAll, "all", "a", false, "export all events regardless of location")
	eventsCmd.AddCommand(eventsOpenCmd, eventsTypesCmd, eventsExportCmd, eventsHookCmd, eventsNotifyCmd)
	rootCmd.AddCommand(eventsCmd)
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`

- [ ] **Step 5: Commit**

```bash
git add cmd/events.go
git commit -m "feat(events): add export, hook, and notify subcommands"
```

---

### Task 6: Config Events Subcommands — cmd/config_events.go

**Files:**
- Create: `cmd/config_events.go`
- Modify: `cmd/config.go`

- [ ] **Step 1: Create cmd/config_events.go**

```go
package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var configEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Configure event filtering and notification settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		method := cfg.Events.NotifyMethod
		if method == "" {
			method = "hook"
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.events.show", map[string]any{
			"LocalRadius":    cfg.Events.EffectiveLocalRadius(),
			"RegionalRadius": cfg.Events.EffectiveRegionalRadius(),
			"NotifyDays":     cfg.Events.EffectiveNotifyDays(),
			"NotifyMethod":   method,
		}))
		return nil
	},
}

var configEventsLocalRadiusCmd = &cobra.Command{
	Use:   "local-radius [km]",
	Short: "Set the local event radius in km",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return configEventsInt(cmd, args, "local_radius",
			func(cfg *config.Config) int { return cfg.Events.EffectiveLocalRadius() },
			func(cfg *config.Config, v int) { cfg.Events.LocalRadius = v },
			"config.events.local_radius.current", "config.events.local_radius.done")
	},
}

var configEventsRegionalRadiusCmd = &cobra.Command{
	Use:   "regional-radius [km]",
	Short: "Set the regional event radius in km",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return configEventsInt(cmd, args, "regional_radius",
			func(cfg *config.Config) int { return cfg.Events.EffectiveRegionalRadius() },
			func(cfg *config.Config, v int) { cfg.Events.RegionalRadius = v },
			"config.events.regional_radius.current", "config.events.regional_radius.done")
	},
}

var configEventsNotifyDaysCmd = &cobra.Command{
	Use:   "notify-days [days]",
	Short: "Set the notification lookahead window in days",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return configEventsInt(cmd, args, "notify_days",
			func(cfg *config.Config) int { return cfg.Events.EffectiveNotifyDays() },
			func(cfg *config.Config, v int) { cfg.Events.NotifyDays = v },
			"config.events.notify_days.current", "config.events.notify_days.done")
	},
}

var validNotifyMethods = []string{"hook", "os", "both"}

var configEventsNotifyMethodCmd = &cobra.Command{
	Use:   "notify-method [hook|os|both]",
	Short: "Set the notification method",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			method := args[0]
			valid := false
			for _, m := range validNotifyMethods {
				if method == m {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "config.events.notify_method.invalid", map[string]any{"Value": method}))
			}
			cfg.Events.NotifyMethod = method
			if err := cfg.Save(paths.ConfigDir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.events.notify_method.done", map[string]any{"Value": method}))
			return nil
		}
		method := cfg.Events.NotifyMethod
		if method == "" {
			method = "hook"
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.events.notify_method.current", map[string]any{"Value": method}))
		return nil
	},
}

func configEventsInt(cmd *cobra.Command, args []string, _ string,
	getter func(*config.Config) int, setter func(*config.Config, int),
	currentKey, doneKey string) error {
	paths, err := xdg.New()
	if err != nil {
		return err
	}
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil {
		return err
	}
	if len(args) == 1 {
		v, err := strconv.Atoi(args[0])
		if err != nil || v <= 0 {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "config.events.invalid_number", map[string]any{"Value": args[0]}))
		}
		setter(cfg, v)
		if err := cfg.Save(paths.ConfigDir); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, doneKey, map[string]any{"Value": v}))
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, currentKey, map[string]any{"Value": getter(cfg)}))
	return nil
}

func init() {
	configEventsCmd.AddCommand(configEventsLocalRadiusCmd, configEventsRegionalRadiusCmd,
		configEventsNotifyDaysCmd, configEventsNotifyMethodCmd)
}
```

- [ ] **Step 2: Register configEventsCmd in cmd/config.go**

In `cmd/config.go`, find the `init()` function and add `configEventsCmd` to the `configCmd.AddCommand(...)` call:

```go
configCmd.AddCommand(configShowCmd, configSetCmd, configCompanyCmd, configTokenCmd, configLocationCmd, configTipRotationCmd, configEventsCmd)
```

Also add events config to `configShowCmd` output. Find the last `fmt.Fprintln` in `configShowCmd`'s `RunE` and add after it:

```go
method := cfg.Events.NotifyMethod
if method == "" {
	method = "hook"
}
fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.events.show", map[string]any{
	"LocalRadius":    cfg.Events.EffectiveLocalRadius(),
	"RegionalRadius": cfg.Events.EffectiveRegionalRadius(),
	"NotifyDays":     cfg.Events.EffectiveNotifyDays(),
	"NotifyMethod":   method,
}))
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add cmd/config_events.go cmd/config.go
git commit -m "feat(events): add config events subcommands for radius and notification settings"
```

---

### Task 7: Smoke Test and Final Verification

- [ ] **Step 1: Full build + vet**

Run: `go build ./... && go vet ./...`
Expected: clean

- [ ] **Step 2: Verify help output**

```bash
./sap-devs events --help
./sap-devs events export --help
./sap-devs events notify --help
./sap-devs config events --help
./sap-devs config events local-radius --help
```

Expected: all show correct descriptions and flags

- [ ] **Step 3: Commit any remaining fixes**

If any issues found, fix and commit.
