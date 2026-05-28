package lexer

import "regexp"

const (
	KindWhitespace TokenKind = iota
	KindNewlines
	KindFunKw
	KindLBracket
	KindRBracket
	KindIntKw
	KindReturnKw
	KindSemicolon
	KindInteger
	KindIdentifier
	KindLParen
	KindRParen
	KindArrow
	KindComma
)

type tokenRule struct {
	kind    TokenKind
	pattern *regexp.Regexp
}

var rules = []tokenRule{
	{KindFunKw, mustAnchor(`fun`)},
	{KindIntKw, mustAnchor(`int`)},
	{KindReturnKw, mustAnchor(`return`)},
	{KindComma, mustAnchor(`,`)},
	{KindArrow, mustAnchor(`->`)},
	{KindLBracket, mustAnchor(`{`)},
	{KindRBracket, mustAnchor(`}`)},
	{KindLParen, mustAnchor(`\(`)},
	{KindRParen, mustAnchor(`\)`)},
	{KindSemicolon, mustAnchor(`;`)},
	{KindInteger, mustAnchor(`-?[0-9]+(_[0-9]+)*`)},
	{KindIdentifier, mustAnchor(`[a-zA-Z_][a-zA-Z0-9_]*`)},
	{KindNewlines, mustAnchor(`\n+`)},
	{KindWhitespace, mustAnchor(`[[:blank:]]+`)},
}

func mustAnchor(pattern string) *regexp.Regexp {
	return regexp.MustCompile("^" + pattern)
}
