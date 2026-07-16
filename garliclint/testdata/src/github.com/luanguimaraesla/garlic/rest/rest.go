package rest

import "net/http"

type Response struct{}

func WriteError(error) *Response           { return &Response{} }
func WriteResponse(int, any) *Response     { return &Response{} }
func WriteMessage(int, string) *Response   { return &Response{} }
func (*Response) Must(http.ResponseWriter) {}
