package httpapi

import (
	"html"
	"net/url"
	"regexp"
	"strings"
)

var (
	siteFooterAnchor = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
	siteFooterHref   = regexp.MustCompile(`(?is)\bhref\s*=\s*"([^"]*)"`)
)

// Only absolute HTTP(S) links are clickable in administrator-authored site footers.
func safeSiteFooterHTML(source string) string {
	return siteFooterAnchor.ReplaceAllStringFunc(source, func(anchor string) string {
		attrs := siteFooterAnchor.FindStringSubmatch(anchor)[1]
		match := siteFooterHref.FindStringSubmatch(attrs)
		if len(match) != 2 || !isHTTPURL(html.UnescapeString(match[1])) {
			parts := siteFooterAnchor.FindStringSubmatch(anchor)
			return parts[2]
		}
		return anchor
	})
}

func isHTTPURL(raw string) bool {
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}
