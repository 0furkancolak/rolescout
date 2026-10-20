package discovery

import (
	"net/url"
	"strings"
)

// SiteVariants returns the other spellings of a company's bare home page, as
// one source or another would have written it: with or without "www.", over
// http or https, with or without the trailing slash. It returns nil for
// anything but a bare home page, whose spellings stand for the same company.
// The pipeline uses it so one site found by DBpedia, a list and a seed is one
// company.
func SiteVariants(raw string) []string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil
	}
	host := strings.ToLower(u.Host)
	bare := strings.TrimPrefix(host, "www.")
	var out []string
	for _, scheme := range []string{"https", "http"} {
		for _, h := range []string{bare, "www." + bare} {
			for _, tail := range []string{"", "/"} {
				if v := scheme + "://" + h + tail; v != raw {
					out = append(out, v)
				}
			}
		}
	}
	return out
}
