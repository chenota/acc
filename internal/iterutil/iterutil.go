package iterutil

import (
	"iter"
)

// Enumerate enumerates an iter.Seq
func Enumerate[T any](seq iter.Seq[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		var i int

		for v := range seq {
			if !yield(i, v) {
				return
			}
			i += 1
		}
	}
}
