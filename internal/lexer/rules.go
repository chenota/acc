package lexer

import "regexp"

const (
	KWhitespace TokenKind = iota
	KNewLines
	KFunKw
	KLBracket
	KRBracket
	KIntKw
	KReturnKw
	KSemicolon
	KInteger
	KIdentifier
	KLParen
	KRParen
	KArrow
	KComma
	KPlus
	KMinus
	KStar
	KDiv
	KLetKw
	KEqual
)

type tokenRule struct {
	kind    TokenKind
	pattern *regexp.Regexp
}

var rules = []tokenRule{
	{KFunKw, mustAnchor(`fun`)},
	{KIntKw, mustAnchor(`int`)},
	{KReturnKw, mustAnchor(`return`)},
	{KLetKw, mustAnchor(`let`)},
	{KComma, mustAnchor(`,`)},
	{KArrow, mustAnchor(`->`)},
	{KLBracket, mustAnchor(`{`)},
	{KRBracket, mustAnchor(`}`)},
	{KLParen, mustAnchor(`\(`)},
	{KRParen, mustAnchor(`\)`)},
	{KPlus, mustAnchor(`\+`)},
	{KMinus, mustAnchor(`-`)},
	{KStar, mustAnchor(`\*`)},
	{KDiv, mustAnchor(`/`)},
	{KEqual, mustAnchor(`=`)},
	{KSemicolon, mustAnchor(`;`)},
	{KInteger, mustAnchor(`-?[0-9]+(_[0-9]+)*`)},
	{KIdentifier, mustAnchor(`[a-zA-Z_][a-zA-Z0-9_]*`)},
	{KNewLines, mustAnchor(`\n+`)},
	{KWhitespace, mustAnchor(`[[:blank:]]+`)},
}

func mustAnchor(pattern string) *regexp.Regexp {
	return regexp.MustCompile("^" + pattern)
}
