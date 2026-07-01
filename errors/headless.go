package errors

// HeadlessErrorT is a code-only error: it carries a kind code and nothing else,
// no message, no cause, no details. It is what an origin reference becomes after
// crossing the wire. The sender strips the sensitive error down to its kind code
// (see [DTO.Origin]) and the receiver rebuilds it as a HeadlessErrorT, so the
// code stays readable for troubleshooting while the body that produced it never
// leaves the sender.
type HeadlessErrorT struct {
	code string
}

// NewHeadlessError returns a HeadlessErrorT for code, or nil when code is empty
// so an absent origin stays absent instead of becoming an empty error.
func NewHeadlessError(code string) error {
	if code == "" {
		return nil
	}

	return &HeadlessErrorT{code}
}

// Error returns the kind code, the only information a headless error holds.
func (h *HeadlessErrorT) Error() string {
	return h.Code()
}

// Code returns the kind code carried by the headless error.
func (h *HeadlessErrorT) Code() string {
	return h.code
}
