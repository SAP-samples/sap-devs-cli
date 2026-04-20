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
