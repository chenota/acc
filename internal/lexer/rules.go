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
	KPlusEq
	KMinusEq
	KMulEq
	KDivEq
	KRef
	KDot
	KNilKw
	KRecKw
	KIfKw
	KElifKw
	KElseKw
	KLoopKw
	KBreakKw
	KContinueKw
	KTrueKw
	KFalseKw
	KBoolKw
	KOr
	KAnd
	KEqEq
	KNotEq
	KLess
	KLessEq
	KGreater
	KGreaterEq
	KNot
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
	{KNilKw, mustAnchor(`nil`)},
	{KRecKw, mustAnchor(`rec`)},
	{KIfKw, mustAnchor(`if`)},
	{KElifKw, mustAnchor(`elif`)},
	{KElseKw, mustAnchor(`else`)},
	{KLoopKw, mustAnchor(`loop`)},
	{KBreakKw, mustAnchor(`break`)},
	{KContinueKw, mustAnchor(`continue`)},
	{KTrueKw, mustAnchor(`true`)},
	{KFalseKw, mustAnchor(`false`)},
	{KBoolKw, mustAnchor(`bool`)},
	{KArrow, mustAnchor(`->`)},
	{KPlusEq, mustAnchor(`\+=`)},
	{KMinusEq, mustAnchor(`-=`)},
	{KMulEq, mustAnchor(`\*=`)},
	{KDivEq, mustAnchor(`/=`)},
	{KOr, mustAnchor(`\|\|`)},
	{KAnd, mustAnchor(`&&`)},
	{KEqEq, mustAnchor(`==`)},
	{KNotEq, mustAnchor(`!=`)},
	{KLessEq, mustAnchor(`<=`)},
	{KGreaterEq, mustAnchor(`>=`)},
	{KLess, mustAnchor(`<`)},
	{KGreater, mustAnchor(`>`)},
	{KNot, mustAnchor(`!`)},
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
	{KRef, mustAnchor(`&`)},
	{KDot, mustAnchor(`\.`)},
	{KComma, mustAnchor(`,`)},
	{KInteger, mustAnchor(`[0-9]+(_[0-9]+)*`)},
	{KIdentifier, mustAnchor(`[a-zA-Z_][a-zA-Z0-9_]*`)},
	{KNewLines, mustAnchor(`\n+`)},
	{KWhitespace, mustAnchor(`[[:blank:]]+`)},
}

func mustAnchor(pattern string) *regexp.Regexp {
	return regexp.MustCompile("^" + pattern)
}
