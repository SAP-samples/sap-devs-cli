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
