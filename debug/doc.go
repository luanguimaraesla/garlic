// Package debug provides developer utilities for inspecting values during
// development.
//
//   - [PrettyPrint] writes indented JSON to stdout.
//   - [PrettyPrintToFile] writes indented JSON to a file.
//   - [PrintToFile] writes compact JSON to a file.
//   - [WriteToFile] writes raw text to a file.
//
// With the "debug" build tag, [Breakpoint] sends SIGTRAP to pause execution
// in the Delve debugger. Do not leave Breakpoint calls in committed code.
package debug
