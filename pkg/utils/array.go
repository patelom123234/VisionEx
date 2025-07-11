package utils

import (
	"cmp"
	"slices"
	"strings"
)

// Map applies a function to each element of a slice and returns a new slice
func Map[T, U any](slice []T, fn func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

// Filter returns a new slice containing only elements that satisfy the predicate
func Filter[T any](slice []T, predicate func(T) bool) []T {
	var result []T
	for _, v := range slice {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

// Find returns the first element that satisfies the predicate, or zero value if not found
func Find[T any](slice []T, predicate func(T) bool) (T, bool) {
	for _, v := range slice {
		if predicate(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// FlatMap applies a function that returns a slice to each element and flattens the result
func FlatMap[T, U any](slice []T, fn func(T) []U) []U {
	var result []U
	for _, v := range slice {
		result = append(result, fn(v)...)
	}
	return result
}

// Some returns true if at least one element satisfies the predicate
func Some[T any](slice []T, predicate func(T) bool) bool {
	for _, v := range slice {
		if predicate(v) {
			return true
		}
	}
	return false
}

// Join concatenates elements of a string slice with a separator
func Join(slice []string, separator string) string {
	if len(slice) == 0 {
		return ""
	}
	if len(slice) == 1 {
		return slice[0]
	}

	// Calculate total length
	n := len(separator) * (len(slice) - 1)
	for _, s := range slice {
		n += len(s)
	}

	// Build the result
	var result strings.Builder
	result.Grow(n)
	result.WriteString(slice[0])
	for _, s := range slice[1:] {
		result.WriteString(separator)
		result.WriteString(s)
	}
	return result.String()
}

// Reduce applies a function against an accumulator and each element in the slice to reduce it to a single value
func Reduce[T, U any](slice []T, fn func(U, T) U, initial U) U {
	result := initial
	for _, v := range slice {
		result = fn(result, v)
	}
	return result
}

// Sort returns a sorted copy of the slice using the standard sort package
func Sort[T cmp.Ordered](slice []T) []T {
	result := make([]T, len(slice))
	copy(result, slice)
	slices.Sort(result)
	return result
}

// Sort for comparable types
func SortComparable[T cmp.Ordered](slice []T) []T {
	result := make([]T, len(slice))
	copy(result, slice)
	slices.Sort(result)
	return result
}

// Contains checks if a slice contains a specific value
func Contains[T comparable](slice []T, value T) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// Concat concatenates multiple slices into a single slice
func Concat[T any](slices ...[]T) []T {
	totalLen := 0
	for _, s := range slices {
		totalLen += len(s)
	}

	result := make([]T, 0, totalLen)
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}
