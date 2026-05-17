package lexer

import "regexp"

const (
	KindSkip TokenKind = iota
	KindFunKw
	KindLBracket
	KindRBracket
	KindIntKw
	KindReturnKw
	KindSemicolon
	KindInteger
	KindIdentifier
)

type tokenRule struct {
	kind    TokenKind
	pattern *regexp.Regexp
}

var rules = []tokenRule{
	{KindFunKw, mustAnchor(`fun`)},
	{KindIntKw, mustAnchor(`int`)},
	{KindReturnKw, mustAnchor(`return`)},
	{KindLBracket, mustAnchor(`{`)},
	{KindRBracket, mustAnchor(`}`)},
	{KindSemicolon, mustAnchor(`;`)},
	{KindInteger, mustAnchor(`-?[0-9][0-9_]*`)},
	{KindIdentifier, mustAnchor(`[a-zA-Z_][a-zA-Z0-9_]*`)},
	{KindSkip, mustAnchor(`[[:blank:]]+`)},
}

func mustAnchor(pattern string) *regexp.Regexp {
	return regexp.MustCompilePOSIX("^" + pattern)
}
