// Package ingest processes media files during asset ingestion.
package ingest

import (
	"context"
	"regexp"
)

type DefaultHandler struct {
	regexps []*regexp.Regexp
}

func NewDefaultHandler(exps []string) DefaultHandler {
	regexps := make([]*regexp.Regexp, len(exps))
	for i, s := range exps {
		regexps[i] = regexp.MustCompile(regexp.QuoteMeta(s))
	}
	return DefaultHandler{regexps}
}

func (h DefaultHandler) Supports(mime string) bool {
	for _, re := range h.regexps {
		if re.MatchString(mime) {
			return true
		}
	}
	return false
}

func (h DefaultHandler) ExtractMeta(_ context.Context, _ string) (FileMeta, error) {
	return FileMeta{}, nil
}
