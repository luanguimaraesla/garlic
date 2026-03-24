// Package toolkit provides generic utility functions for pointer handling and
// nil checking.
//
//   - [PointerOf] returns a pointer to its argument, useful for inline literal
//     pointers: toolkit.PointerOf("hello") returns *string.
//   - [ValueOrDefault] safely dereferences a pointer, returning the zero value
//     of T when the pointer is nil.
//   - [IsValueNil] checks whether an interface value is nil, correctly handling
//     typed nil pointers, slices, maps, channels, and functions.
package toolkit
