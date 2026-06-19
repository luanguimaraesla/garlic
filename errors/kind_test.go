//go:build unit

package errors

import "testing"

func TestLookupByCode_hit(t *testing.T) {
	k, ok := LookupByCode(KindNotFoundError.Code)
	if !ok || k != KindNotFoundError {
		t.Errorf("LookupByCode(%q) = %v, %v; want KindNotFoundError, true", KindNotFoundError.Code, k, ok)
	}
}

func TestLookupByCode_miss(t *testing.T) {
	k, ok := LookupByCode("DOES-NOT-EXIST")
	if ok || k != nil {
		t.Errorf("LookupByCode(miss) = %v, %v; want nil, false", k, ok)
	}
}
