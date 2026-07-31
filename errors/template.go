package errors

// TemplateT is a structure that defines a template for creating and propagating errors.
// It includes a kind, a message, and optional parameters (opts) that can be used to
// customize the error. The Template function initializes a new TemplateT instance.
// The New method creates a new error based on the template, while the Propagate method
// propagates an existing error with additional context provided by the template.
type TemplateT struct {
	kind    *Kind
	message string
	opts    []Opt
}

// Template initializes a new TemplateT instance, which is used to define a template
// for creating and propagating errors. It takes a kind, a message, and optional parameters
// (opts) that can be used to customize the error. This function returns a pointer to the
// TemplateT struct, allowing for the creation of new errors or the propagation of existing
// errors with additional context provided by the template.
func Template(kind *Kind, message string, opts ...Opt) *TemplateT {
	return &TemplateT{
		kind:    kind,
		message: message,
		opts:    opts,
	}
}

// New creates a new error based on the template, using the specified kind, message,
// and optional parameters (opts). It combines the template's options with any additional
// options provided, allowing for customization of the error. This method returns a pointer
// to an ErrorT instance, representing the newly created error.
func (t *TemplateT) New(opts ...Opt) *ErrorT {
	combined := make([]Opt, 0, len(t.opts)+len(opts))
	combined = append(combined, t.opts...)
	combined = append(combined, opts...)
	return New(t.kind, t.message, combined...)
}

// Propagate propagates an existing error with the kind, message, and options of
// the template. Caller options are applied after the template's, so they win on
// conflict.
//
// When err is a nil error interface, Propagate returns nil and neither the
// template nor the caller options run. A non-nil result is always backed by
// *ErrorT. A typed nil stored inside an error interface is not nil and is not
// normalized.
func (t *TemplateT) Propagate(err error, opts ...Opt) error {
	combined := make([]Opt, 0, len(t.opts)+len(opts))
	combined = append(combined, t.opts...)
	combined = append(combined, opts...)
	return PropagateAs(t.kind, err, t.message, combined...)
}
