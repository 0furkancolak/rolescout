// Package discovery finds companies with public career pages and identifies
// the job board system behind each page.
package discovery

import (
	"net/url"
	"regexp"
	"strings"
)

// Board is a company's job board on a supported system, with the adapter
// config that reads it.
type Board struct {
	ATS    string
	ID     string
	Config map[string]any
}

var (
	idPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$`)
	labelPattern  = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	localePattern = regexp.MustCompile(`^[a-z]{2}-[A-Z]{2}$`)
	workdayHost   = regexp.MustCompile(`^([a-z0-9-]+)\.wd[0-9]+\.myworkdayjobs\.com$`)
	keyUnsafe     = regexp.MustCompile(`[^a-z0-9_]+`)
)

// reserved are path segments or subdomains that are not company boards.
var reserved = map[string]bool{
	"embed": true, "api": true, "v0": true, "v1": true, "static": true, "assets": true, "j": true,
	"www": true, "app": true, "careers": true, "jobs": true, "login": true, "help": true, "wday": true,
}

// ParseBoardURL recognises a URL on a supported job board system and returns
// the board it belongs to.
func ParseBoardURL(raw string) (Board, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Board{}, false
	}
	host := strings.ToLower(u.Hostname())
	segs := segments(u.Path)
	first := ""
	if len(segs) > 0 {
		first = segs[0]
	}

	switch {
	case host == "boards.greenhouse.io" || host == "job-boards.greenhouse.io" ||
		host == "boards.eu.greenhouse.io" || host == "job-boards.eu.greenhouse.io":
		if first == "embed" {
			return board("greenhouse", u.Query().Get("for"), "board_token")
		}
		return board("greenhouse", first, "board_token")
	case host == "jobs.lever.co":
		return board("lever", first, "site")
	case host == "jobs.eu.lever.co":
		b, ok := board("lever", first, "site")
		if ok {
			b.Config["region"] = "eu"
		}
		return b, ok
	case host == "jobs.ashbyhq.com":
		return board("ashby", first, "board")
	case host == "apply.workable.com":
		return board("workable", first, "account")
	case strings.HasSuffix(host, ".workable.com"):
		return subdomainBoard("workable", strings.TrimSuffix(host, ".workable.com"), "account")
	case strings.HasSuffix(host, ".recruitee.com"):
		return subdomainBoard("recruitee", strings.TrimSuffix(host, ".recruitee.com"), "subdomain")
	case strings.HasSuffix(host, ".teamtailor.com"):
		return subdomainBoard("teamtailor", strings.TrimSuffix(host, ".teamtailor.com"), "subdomain")
	case strings.HasSuffix(host, ".breezy.hr"):
		return subdomainBoard("breezy", strings.TrimSuffix(host, ".breezy.hr"), "subdomain")
	case host == "ats.rippling.com":
		return board("rippling", first, "board")
	case host == "jobs.gem.com":
		return board("gem", first, "board")
	case strings.HasSuffix(host, ".jobs.personio.de"), strings.HasSuffix(host, ".jobs.personio.com"):
		tld := host[strings.LastIndex(host, ".")+1:]
		b, ok := subdomainBoard("personio", strings.TrimSuffix(host, ".jobs.personio."+tld), "subdomain")
		if ok {
			b.Config["tld"] = tld
		}
		return b, ok
	case strings.HasSuffix(host, ".bamboohr.com"):
		if first != "careers" && first != "jobs" {
			return Board{}, false
		}
		return subdomainBoard("bamboohr", strings.TrimSuffix(host, ".bamboohr.com"), "subdomain")
	case workdayHost.MatchString(host):
		tenant := workdayHost.FindStringSubmatch(host)[1]
		if len(segs) > 0 && localePattern.MatchString(segs[0]) {
			segs = segs[1:]
		}
		if len(segs) == 0 || reserved[strings.ToLower(segs[0])] || !idPattern.MatchString(segs[0]) {
			return Board{}, false
		}
		site := segs[0]
		return Board{ATS: "workday", ID: tenant + "/" + site, Config: map[string]any{"host": host, "tenant": tenant, "site": site}}, true
	}
	return Board{}, false
}

func segments(path string) []string {
	var out []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func board(ats, id, field string) (Board, bool) {
	if id == "" || reserved[strings.ToLower(id)] || !idPattern.MatchString(id) {
		return Board{}, false
	}
	return Board{ATS: ats, ID: id, Config: map[string]any{field: id}}, true
}

func subdomainBoard(ats, sub, field string) (Board, bool) {
	if strings.Contains(sub, ".") || reserved[sub] || !labelPattern.MatchString(sub) {
		return Board{}, false
	}
	return Board{ATS: ats, ID: sub, Config: map[string]any{field: sub}}, true
}

// SourceKey is the job source key for the board, such as
// "company-greenhouse-acme".
func (b Board) SourceKey() string {
	id := strings.Trim(keyUnsafe.ReplaceAllString(strings.ToLower(b.ID), "-"), "-")
	key := "company-" + b.ATS + "-" + id
	if len(key) > 120 {
		key = key[:120]
	}
	return key
}
