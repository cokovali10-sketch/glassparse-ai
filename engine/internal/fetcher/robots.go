// robots.txt support: parse a site's robots file, decide whether a given path
// is allowed for a user-agent, and honour Crawl-delay. Respecting robots lets
// crawlers stay polite and is table stakes for any sitemap / crawl tooling.
package fetcher

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// robotsRule is a single Allow/Disallow line with glob pattern support.
type robotsRule struct {
	disallow bool
	pattern  string
}

// Robots holds the parsed, per-agent rules of a site's robots.txt.
type Robots struct {
	groups   map[string]*robotsGroup // group name -> rules (lower-cased UA)
	sitemaps []string
}

type robotsGroup struct {
	agent      string
	rules      []robotsRule
	crawlDelay time.Duration
}

// ParseRobots parses robots.txt content and returns per-user-agent rules.
func ParseRobots(content string) *Robots {
	r := &Robots{groups: map[string]*robotsGroup{}}
	lines := strings.Split(content, "\n")
	var cur *robotsGroup
	flush := func() { cur = nil }
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		switch key {
		case "user-agent":
			flush() // start a new group on a User-agent directive.
			agent := strings.ToLower(val)
			cur = &robotsGroup{agent: agent}
			r.groups[agent] = cur
		case "allow", "disallow":
			if cur == nil {
				continue
			}
			cur.rules = append(cur.rules, robotsRule{
				disallow: key == "disallow",
				pattern:  val,
			})
		case "crawl-delay":
			if cur == nil {
				continue
			}
			if n, err := strconv.ParseFloat(val, 64); err == nil && n > 0 {
				cur.crawlDelay = time.Duration(n * float64(time.Second))
			}
		case "sitemap":
			r.sitemaps = append(r.sitemaps, val)
		}
	}
	return r
}

// groupFor returns the most specific group for a UA (exact name normally; falls
// back to "*"). Groups are greedy on the first matching section, which matches
// most real-world robots files.
func (r *Robots) groupFor(ua string) *robotsGroup {
	ua = strings.ToLower(ua)
	if g, ok := r.groups[ua]; ok {
		return g
	}
	return r.groups["*"]
}

// sitemap returns the pattern ("" means no agent-specific lines matched).
func (r *Robots) CrawlDelay(ua string) time.Duration {
	if g := r.groupFor(ua); g != nil {
		return g.crawlDelay
	}
	return 0
}

// Sitemaps lists every Sitemap: line found in the file.
func (r *Robots) Sitemaps() []string {
	return r.sitemaps
}

// Allowed reports whether the given path may be fetched by the given UA.
// It follows the standard "longest-matching rule wins" semantics: if no rule
// touches the path it is allowed; if the longest rule is a Disallow, deny.
func (r *Robots) Allowed(path string, ua string) bool {
	g := r.groupFor(ua)
	var best *robotsRule
	bestLen := -1
	if g != nil {
		for i := range g.rules {
			rule := &g.rules[i]
			if robotsMatch(rule.pattern, path) && len(rule.pattern) > bestLen {
				best, bestLen = rule, len(rule.pattern)
			}
		}
	}
	return best == nil || !best.disallow
}

// robotsMatch compares a robots pattern — with `*` wildcards and an optional
// trailing `$` end-anchor — to a path. Unanchored patterns use prefix semantics:
// the pattern must match the *start* of the path (so "/nogoogle" blocks
// "/nogoogle/x"), which mirrors how real-world robots.txt rules behave.
func robotsMatch(pattern, path string) bool {
	if pattern == "" || pattern == "/" {
		return true
	}
	anchored := strings.HasSuffix(pattern, "$")
	if anchored {
		pattern = strings.TrimSuffix(pattern, "$")
	}
	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			sb.WriteString(".*")
		default:
			sb.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	if !anchored {
		// prefix semantics: allow trailing characters to the end of the path.
		sb.WriteString(".*")
	}
	sb.WriteString("$")
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return false
	}
	return re.MatchString(path)
}

// FetchRobots downloads and parses a site's robots.txt (best-effort).
func (c *Client) FetchRobots(ctx context.Context, pageURL string, optUA string) (*Robots, error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}
	robotsURL := u.Scheme + "://" + u.Host + "/robots.txt"
	ua := c.UserAgent
	if optUA != "" {
		ua = optUA
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil // no robots file → allow everything
	}
	b := &strings.Builder{}
	_, _ = io.Copy(b, io.LimitReader(resp.Body, 1<<20))
	return ParseRobots(b.String()), nil
}