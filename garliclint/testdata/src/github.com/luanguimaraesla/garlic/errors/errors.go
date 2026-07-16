package errors

import "go.uber.org/zap"

type Opt interface{ Opt(*ErrorT) }

type ErrorT struct{}

type ContextT struct{}

type Kind struct{}

type TemplateT struct{}

type field struct{}

var KindError = &Kind{}
var KindSystemError = &Kind{}
var KindUserError = &Kind{}

func (*ErrorT) Error() string                      { return "" }
func (*ErrorT) With(...Opt) *ErrorT                { return &ErrorT{} }
func (*TemplateT) New(...Opt) *ErrorT              { return &ErrorT{} }
func (*TemplateT) Propagate(error, ...Opt) *ErrorT { return &ErrorT{} }

func Propagate(error, string, ...Opt) *ErrorT          { return &ErrorT{} }
func PropagateAs(*Kind, error, string, ...Opt) *ErrorT { return &ErrorT{} }
func New(*Kind, string, ...Opt) *ErrorT                { return &ErrorT{} }
func Raw(*Kind, string, ...Opt) *ErrorT                { return &ErrorT{} }
func From(*Kind, error, string, ...Opt) *ErrorT        { return &ErrorT{} }
func Override(*Kind, error, string, ...Opt) *ErrorT    { return &ErrorT{} }
func Mirror(*Kind, ...Opt) *ErrorT                     { return &ErrorT{} }
func MirrorOverride(*Kind, error, ...Opt) *ErrorT      { return &ErrorT{} }
func Zap(error) zap.Field                              { return zap.Field{} }
func Context(...Opt) *ContextT                         { return &ContextT{} }
func Field(string, any) Opt                            { return field{} }
func Template(*Kind, string, ...Opt) *TemplateT        { return &TemplateT{} }

func (field) Opt(*ErrorT) {}
