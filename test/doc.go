//go:build unit

// Package test provides a builder-pattern HTTP test case utility for unit
// testing Chi-based handlers.
//
// # APITestCase
//
// Build a test case by chaining methods on [APITestCase]:
//
//	test.New(t, "get user by id").
//	    Handler(myHandler).
//	    Get("/users/{id}").
//	    Param("id", "abc-123").
//	    ExpectStatus(http.StatusOK).
//	    ExpectMessage("ok").
//	    End()
//
// Available request methods: [APITestCase.Get], [APITestCase.Post],
// [APITestCase.Put], [APITestCase.Delete]. Set path parameters with
// [APITestCase.Param], query parameters with [APITestCase.Query], and a JSON
// body with [APITestCase.Body].
//
// Assertions include [APITestCase.ExpectStatus], [APITestCase.ExpectResponse]
// (raw JSON), [APITestCase.ExpectMessage], and [APITestCase.ExpectObject].
// Call [APITestCase.End] to execute the request and run all assertions.
//
// [AssertModelJsonFields] compares the JSON serialization of a value against
// an expected JSON string.
package test
