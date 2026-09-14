package iterutil

import (
	"iter"
	"slices"
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

func Reverse[T any](seq iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		l := slices.Collect(seq)

		for _, v := range slices.Backward(l) {
			if !yield(v) {
				return
			}
		}
	}
}

func Second[T1, T2 any](seq iter.Seq2[T1, T2]) iter.Seq[T2] {
	return func(yield func(T2) bool) {
		for _, v := range seq {
			if !yield(v) {
				return
			}
		}
	}
}
