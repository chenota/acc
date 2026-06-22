package diagnostic

import (
	"errors"
	"fmt"
	"io"
)

type Pos struct {
	File string
	Line int
	Col  int
}

func (p Pos) GreaterThan(v Pos) bool {
	if p.Line == v.Line {
		return p.Col > v.Col
	}
	return p.Line > v.Line
}

type Error struct {
	pos     Pos
	message string
}

func (e *Error) Error() string {
	if e == nil {
		return "unknown error"
	}
	return fmt.Sprintf("%s:%d:%d: error: %s", e.pos.File, e.pos.Line, e.pos.Col, e.message)
}

func (e *Error) Message() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *Error) Pos() Pos {
	if e == nil {
		return Pos{}
	}
	return e.pos
}

func NewError(pos Pos, format string, args ...any) *Error {
	return &Error{
		message: fmt.Sprintf(format, args...),
		pos:     pos,
	}
}

func PrintError(w io.Writer, err error) {
	if e, ok := errors.AsType[*Error](err); ok {
		fmt.Fprintln(w, e.Error())
		return
	}
	fmt.Fprintf(w, "error: %s\n", err.Error())
}
