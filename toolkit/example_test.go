package toolkit_test

import (
	"fmt"

	"github.com/luanguimaraesla/garlic/toolkit"
)

func ExamplePointerOf() {
	p := toolkit.PointerOf(42)
	fmt.Println(*p)
	// Output:
	// 42
}

func ExampleValueOrDefault() {
	s := "hello"
	fmt.Println(toolkit.ValueOrDefault(&s))
	fmt.Println(toolkit.ValueOrDefault[string](nil))
	// Output:
	// hello
	//
}

func ExampleIsValueNil() {
	var p *int
	fmt.Println(toolkit.IsValueNil(nil))
	fmt.Println(toolkit.IsValueNil(p))
	fmt.Println(toolkit.IsValueNil(42))
	// Output:
	// true
	// true
	// false
}
