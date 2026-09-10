//go:build unit

package errors

import "testing"

func TestDetails_MergesIntoExistingDetails(t *testing.T) {
	e := New(KindError, "boom",
		Hint("keep me"),
		Details(map[string]any{"attempt": 2, "resource": "order"}),
	)

	if e.Details["hint"] != "keep me" {
		t.Errorf("hint = %v, want the unrelated key to survive", e.Details["hint"])
	}
	if e.Details["attempt"] != 2 || e.Details["resource"] != "order" {
		t.Errorf("details = %v, want the merged fields", e.Details)
	}
}

func TestDetails_LaterEntriesWin(t *testing.T) {
	e := New(KindError, "boom",
		Details(map[string]any{"attempt": 1, "resource": "order"}),
		Details(map[string]any{"attempt": 2}),
	)

	if e.Details["attempt"] != 2 {
		t.Errorf("attempt = %v, want the later option to win", e.Details["attempt"])
	}
	if e.Details["resource"] != "order" {
		t.Errorf("resource = %v, want it untouched", e.Details["resource"])
	}
}

func TestDetails_AcceptsNilAndEmptyMaps(t *testing.T) {
	for name, fields := range map[string]map[string]any{
		"nil":   nil,
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			e := New(KindError, "boom", Hint("kept"), Details(fields))

			if len(e.Details) != 1 || e.Details["hint"] != "kept" {
				t.Errorf("details = %v, want only the hint", e.Details)
			}
		})
	}
}

func TestDetails_InitializesANilDestination(t *testing.T) {
	e := &ErrorT{}

	Details(map[string]any{"attempt": 1}).Opt(e)

	if e.Details["attempt"] != 1 {
		t.Errorf("details = %v, want the option to allocate the map", e.Details)
	}
}

// The option snapshots its input, so it can be reused and the caller stays free
// to keep writing to the map it passed.
func TestDetails_SnapshotsAndIsolatesItsInput(t *testing.T) {
	fields := map[string]any{"attempt": 1}
	opt := Details(fields)

	first := New(KindError, "first", opt)

	fields["attempt"] = 99
	fields["late"] = true

	second := New(KindError, "second", opt)
	second.Details["attempt"] = 7

	if first.Details["attempt"] != 1 {
		t.Errorf("first attempt = %v, want the value captured at option time", first.Details["attempt"])
	}
	if _, ok := first.Details["late"]; ok {
		t.Error("a key added after the option was built should not reach the error")
	}
	if second.Details["attempt"] != 7 {
		t.Errorf("second attempt = %v, want the error's own write", second.Details["attempt"])
	}
	if first.Details["attempt"] != 1 {
		t.Error("writing to one error's details must not reach another built from the same option")
	}
	if fields["attempt"] != 99 {
		t.Errorf("the caller's map = %v, want it untouched by the errors", fields)
	}
}
