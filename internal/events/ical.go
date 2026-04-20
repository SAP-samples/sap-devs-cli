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
