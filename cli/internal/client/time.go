package client

import (
	"encoding/json"
	"time"
)

// Time is a time.Time that tolerates the various timestamp shapes the Mem0
// OSS API emits: RFC3339 strings, "2006-01-02 15:04:05", or null. When the
// value cannot be parsed it degrades to the zero time instead of failing the
// whole payload decode.
type Time struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *Time) UnmarshalJSON(b []byte) error {
	if string(b) == "null" || string(b) == `""` {
		t.Time = time.Time{}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		parsed, err := parseTimestamp(s)
		if err != nil {
			// Best effort: keep the raw string so we don't block decoding.
			t.Time = time.Time{}
			return nil
		}
		t.Time = parsed
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err == nil && f > 0 {
		sec := int64(f)
		nsec := int64((f - float64(sec)) * 1e9)
		t.Time = time.Unix(sec, nsec).UTC()
		return nil
	}
	t.Time = time.Time{}
	return nil
}

// MarshalJSON implements json.Marshaler.
func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Format(time.RFC3339))
}

func parseTimestamp(s string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, &time.ParseError{}
}
