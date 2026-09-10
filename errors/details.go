package errors

// Details snapshots fields at the top level and merges them into public error
// details; later options win. Use [Context] for private diagnostics.
func Details(fields map[string]any) Opt {
	snapshot := make(map[string]any, len(fields))
	for key, value := range fields {
		snapshot[key] = value
	}

	return details(snapshot)
}

type details map[string]any

func (d details) Opt(e *ErrorT) {
	if e.Details == nil {
		e.Details = make(map[string]any, len(d))
	}

	for key, value := range d {
		e.Details[key] = value
	}
}
