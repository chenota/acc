package iterutil

import (
	"iter"
)

// Reverse2 reverses an iter.Seq2
func Reverse2[T1, T2 any](seq iter.Seq2[T1, T2]) iter.Seq2[T1, T2] {
	var t1s []T1
	var t2s []T2

	for t1, t2 := range seq {
		t1s = append(t1s, t1)
		t2s = append(t2s, t2)
	}

	return func(yield func(T1, T2) bool) {
		for i := len(t1s) - 1; i >= 0; i -= 1 {
			if !yield(t1s[i], t2s[i]) {
				return
			}
		}
	}
}

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
