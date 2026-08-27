// Package slug generates URL-safe slugs.
package slug

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	reNonSlug     = regexp.MustCompile(`[^a-z0-9-]`)
	reMultiHyphen = regexp.MustCompile(`-{2,}`)
)

// ToSlug converts a folder name to a URL-safe routing slug.
// "Campaign Photos" → "campaign-photos"
// "Q2 / Summer 2026" → "q2-summer-2026".
func ToSlug(name string) string {
	// Normalise unicode, decompose accented characters
	name = norm.NFD.String(name)
	// Strip non-ASCII combining characters (accents)
	var b strings.Builder
	for _, r := range name {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	s := strings.ToLower(b.String())
	s = strings.NewReplacer(" ", "-", "_", "-", "/", "-").Replace(s)
	s = reNonSlug.ReplaceAllString(s, "")
	s = reMultiHyphen.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// ParseSubaddress splits a local-part like "ws_abc123+brand-assets"
// into token "ws_abc123" and tag "brand-assets".
// Returns tag = "" if no subaddress is present or the tag is empty.
func ParseSubaddress(localPart string) (token, tag string) {
	before, after, ok := strings.Cut(localPart, "+")
	if !ok {
		return localPart, ""
	}
	tag = strings.ToLower(strings.TrimSpace(after))
	return before, tag
}
