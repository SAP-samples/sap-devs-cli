package content

import "strings"

// FlattenEventTypes collects all event types from all packs.
func FlattenEventTypes(packs []*Pack) []EventType {
	var out []EventType
	for _, p := range packs {
		out = append(out, p.EventTypes...)
	}
	return out
}

// FlattenEventInstances collects all event instances from all packs.
func FlattenEventInstances(packs []*Pack) []EventInstance {
	var out []EventInstance
	for _, p := range packs {
		out = append(out, p.EventInstances...)
	}
	return out
}

// FilterEventsByType returns events matching the given type ID.
func FilterEventsByType(events []EventInstance, typeID string) []EventInstance {
	var out []EventInstance
	for _, e := range events {
		if e.Type == typeID {
			out = append(out, e)
		}
	}
	return out
}

// FindEvent returns a pointer to the first event with an exact ID match, or nil.
func FindEvent(events []EventInstance, id string) *EventInstance {
	for i := range events {
		if events[i].ID == id {
			return &events[i]
		}
	}
	return nil
}

// FilterEventsByQuery returns events matching query in title, location, or tags.
func FilterEventsByQuery(events []EventInstance, query string) []EventInstance {
	q := strings.ToLower(query)
	var out []EventInstance
	for _, e := range events {
		if strings.Contains(strings.ToLower(e.Title), q) ||
			strings.Contains(strings.ToLower(e.Location), q) {
			out = append(out, e)
			continue
		}
		for _, tag := range e.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}
