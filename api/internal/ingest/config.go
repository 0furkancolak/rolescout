package ingest

import (
	"fmt"
	"regexp"
	"slices"
)

// Field describes one adapter config field for CheckConfig.
type Field struct {
	Name     string
	Pattern  *regexp.Regexp // required format of a string value
	OneOf    []string       // allowed values, instead of Pattern
	Optional bool
}

// BoardID matches the board, site or account identifiers ATS vendors use.
var BoardID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$`)

// Subdomain matches a single DNS label.
var Subdomain = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

// CheckConfig validates an adapter config: every field is a string of the
// given format, required fields are present, and unknown fields are
// rejected. Every adapter also accepts an optional non-empty "company"
// display name.
func CheckConfig(config map[string]any, fields ...Field) error {
	known := map[string]Field{"company": {Name: "company", Pattern: regexp.MustCompile(`\S`), Optional: true}}
	for _, f := range fields {
		known[f.Name] = f
	}
	for key, raw := range config {
		f, ok := known[key]
		if !ok {
			return fmt.Errorf("unknown config field %q", key)
		}
		s, ok := raw.(string)
		if !ok || s == "" {
			return fmt.Errorf("config field %q must be a non-empty string", key)
		}
		if f.Pattern != nil && !f.Pattern.MatchString(s) {
			return fmt.Errorf("config field %q has an invalid value", key)
		}
		if f.OneOf != nil && !slices.Contains(f.OneOf, s) {
			return fmt.Errorf("config field %q must be one of %v", key, f.OneOf)
		}
	}
	for _, f := range known {
		if _, ok := config[f.Name]; !ok && !f.Optional {
			return fmt.Errorf("config field %q is required", f.Name)
		}
	}
	return nil
}

// ConfigString returns a string config value or "".
func ConfigString(config map[string]any, key string) string {
	s, _ := config[key].(string)
	return s
}
