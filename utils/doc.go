// Package utils provides struct transformation utilities.
//
// [FlattenStruct] recursively flattens a nested struct into a flat
// map[string]interface{} using mapstructure tags as keys. Nested keys are
// joined with a dot separator:
//
//	type Address struct {
//	    City string `mapstructure:"city"`
//	}
//	type User struct {
//	    Name    string  `mapstructure:"name"`
//	    Address Address `mapstructure:"address"`
//	}
//	// FlattenStruct(user) => {"name": "Alice", "address.city": "NYC"}
package utils
