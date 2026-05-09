package lexer

import "regexp"

const (
	KindSkip TokenKind = iota
	KindFunKw
)

type tokenRule struct {
	kind    TokenKind
	pattern *regexp.Regexp
}

var rules = []tokenRule{
	{KindFunKw, mustAnchor(`fun`)},
	{KindSkip, mustAnchor(`[[:blank:]]+`)},
}

func mustAnchor(pattern string) *regexp.Regexp {
	return regexp.MustCompilePOSIX("^" + pattern)
}
