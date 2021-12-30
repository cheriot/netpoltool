package util

import (
	. "constraints"
)

type Option[T any] struct {
	value *T
}

func Some[T any](t T) Option[T] {
	return Option[T]{value: &t}
}

func None[T any]() Option[T] {
	return Option[T]{value: nil}
}

func (o Option[T]) Map(f func(t T) any) any {
	if o.value != nil {
		foo := f(*o.value)
		return Some[any](foo)
	}
	return o // None
}

func Contains[T comparable](slice []T, t0 T) bool {
	for _, t := range slice {
		if t0 == t {
			return true
		}
	}
	return false
}

func Map[A any, B any](slice []A, f func(A) B) []B {
	bs := make([]B, 0, len(slice))
	for _, a := range slice {
		bs = append(bs, f(a))
	}
	return bs
}

func Filter[T any](slice []T, filter func(t0 T) bool) []T {
	filtered := make([]T, 0, len(slice))
	for _, t := range slice {
		if filter(t) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func Fold[TVal any, TAcc any](slice []TVal, acc0 TAcc, f func(t0 TAcc, t1 TVal) TAcc) TAcc {
	acc := acc0
	for _, tval := range slice {
		acc = f(acc, tval)
	}
	return acc
}

func Max[T Ordered](t0, t1 T) T {
	if t1 > t0 {
		return t1
	}
	return t0
}

func Min[T Ordered](t0, t1 T) T {
	if t1 < t0 {
		return t1
	}
	return t0
}
