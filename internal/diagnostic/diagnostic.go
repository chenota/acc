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

type Error struct {
	pos     Pos
	message string
}

func (e *Error) Error() string {
	return e.message
}

func (e *Error) Pos() Pos {
	return e.pos
}

func NewError(message string, pos Pos) error {
	return &Error{
		message: message,
		pos:     pos,
	}
}

func PrintError(w io.Writer, err error) {
	if poser, ok := errors.AsType[interface {
		Error() string
		Pos() Pos
	}](err); ok {
		pos := poser.Pos()
		fmt.Fprintf(w, "%s:%d:%d: error: %s\n", pos.File, pos.Line, pos.Col, poser.Error())
		return
	}
	fmt.Fprintf(w, "unknown: error: %s\n", err.Error())
}
