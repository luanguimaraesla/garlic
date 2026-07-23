//go:build !unit

package unittesttag // want "\\[G8.01\\]"

import "testing"

func TestNegatedTag(*testing.T) {}
