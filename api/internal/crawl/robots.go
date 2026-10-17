package crawl

import (
	"bufio"
	"bytes"
	"strings"
)

// robotsRules holds the rules of the robots.txt group that applies to us
// (RFC 9309). The zero value allows everything.
type robotsRules struct {
	rules []robotsRule
	// disallowAll is set when robots.txt could not be read because of a
	// server error; RFC 9309 says to assume complete disallow then.
	disallowAll bool
}

type robotsRule struct {
	allow   bool
	pattern string
}

// parseRobots returns the rules for agent: the groups naming its product token
// (case-insensitively) or, when none does, the "*" groups.
func parseRobots(body []byte, agent string) robotsRules {
	agent = strings.ToLower(agent)
	var specific, star []robotsRule
	var agents []string
	inRules := false
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "user-agent":
			if inRules {
				agents, inRules = nil, false
			}
			agents = append(agents, strings.ToLower(value))
		case "allow", "disallow":
			inRules = true
			if value == "" {
				// An empty Disallow allows everything; an empty Allow says nothing.
				continue
			}
			rule := robotsRule{allow: key == "allow", pattern: value}
			for _, a := range agents {
				switch a {
				case agent:
					specific = append(specific, rule)
				case "*":
					star = append(star, rule)
				}
			}
		}
	}
	matched := false
	for _, a := range agentsInFile(body) {
		if a == agent {
			matched = true
		}
	}
	if matched {
		return robotsRules{rules: specific}
	}
	return robotsRules{rules: star}
}

func agentsInFile(body []byte) []string {
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		key, value, ok := strings.Cut(sc.Text(), ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), "user-agent") {
			if i := strings.IndexByte(value, '#'); i >= 0 {
				value = value[:i]
			}
			out = append(out, strings.ToLower(strings.TrimSpace(value)))
		}
	}
	return out
}

// allowed reports whether path (with query) may be fetched. The longest
// matching rule wins; on a tie Allow wins.
func (r robotsRules) allowed(path string) bool {
	if r.disallowAll {
		return false
	}
	best, allow := -1, true
	for _, rule := range r.rules {
		if !matchPattern(rule.pattern, path) {
			continue
		}
		n := len(rule.pattern)
		if n > best || (n == best && rule.allow) {
			best, allow = n, rule.allow
		}
	}
	return allow
}

// matchPattern matches a robots.txt path pattern, where "*" matches any
// sequence and a trailing "$" anchors the end.
func matchPattern(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	if anchored {
		pattern = pattern[:len(pattern)-1]
	}
	return glob(pattern, path, anchored)
}

func glob(pattern, s string, anchored bool) bool {
	for len(pattern) > 0 {
		if pattern[0] == '*' {
			pattern = strings.TrimLeft(pattern, "*")
			if pattern == "" {
				return true
			}
			for i := 0; i <= len(s); i++ {
				if glob(pattern, s[i:], anchored) {
					return true
				}
			}
			return false
		}
		if s == "" || s[0] != pattern[0] {
			return false
		}
		pattern, s = pattern[1:], s[1:]
	}
	return !anchored || s == ""
}
