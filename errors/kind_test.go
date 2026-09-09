//go:build unit

package errors

import (
	"net/http"
	"sync"
	"testing"
)

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

func TestCustomizeStatusCode_copiesTheKindAndKeepsItsIdentity(t *testing.T) {
	custom := KindNotFoundError.CustomizeStatusCode(599)

	if custom == KindNotFoundError {
		t.Fatal("CustomizeStatusCode should return a copy, not the receiver")
	}
	if got := custom.StatusCode(); got != 599 {
		t.Errorf("custom.StatusCode() = %d, want 599", got)
	}
	if got := KindNotFoundError.StatusCode(); got != http.StatusNotFound {
		t.Errorf("the receiver now reports %d, want its own 404", got)
	}

	if custom.Name != KindNotFoundError.Name ||
		custom.Code != KindNotFoundError.Code ||
		custom.Description != KindNotFoundError.Description ||
		custom.Parent != KindNotFoundError.Parent {
		t.Errorf("the copy %+v does not carry the identity of %+v", custom, KindNotFoundError)
	}
	if custom.FQN() != KindNotFoundError.FQN() {
		t.Errorf("FQN() = %q, want %q", custom.FQN(), KindNotFoundError.FQN())
	}
	if !custom.Is(KindNotFoundError) || !KindNotFoundError.Is(custom) {
		t.Error("a customized kind should still match the kind it copied")
	}
	if !custom.Is(KindUserError) || custom.Is(KindSystemError) {
		t.Error("ancestor matching should survive a customization")
	}
}

func TestCustomizeStatusCode_ignoresNonPositiveCodes(t *testing.T) {
	for _, code := range []int{0, -1, -599} {
		if got := KindNotFoundError.CustomizeStatusCode(code); got != KindNotFoundError {
			t.Errorf("CustomizeStatusCode(%d) = %v, want the receiver unchanged", code, got)
		}
	}
}

func TestCustomizeStatusCode_isRepeatableAndSiblingsAreIndependent(t *testing.T) {
	first := KindNotFoundError.CustomizeStatusCode(http.StatusBadGateway)
	second := first.CustomizeStatusCode(599)
	sibling := KindNotFoundError.CustomizeStatusCode(http.StatusTeapot)

	if got := first.StatusCode(); got != http.StatusBadGateway {
		t.Errorf("first.StatusCode() = %d, want 502", got)
	}
	if got := second.StatusCode(); got != 599 {
		t.Errorf("second.StatusCode() = %d, want 599", got)
	}
	if got := sibling.StatusCode(); got != http.StatusTeapot {
		t.Errorf("sibling.StatusCode() = %d, want 418", got)
	}
	if second.Code != KindNotFoundError.Code || !second.Is(KindNotFoundError) {
		t.Error("customizing a customized kind should keep the original identity")
	}
}

func TestCustomizeStatusCode_leavesTheRegistryAndTemplatesAlone(t *testing.T) {
	registered := GetByCode(KindNotFoundError.Code)
	template := Template(KindNotFoundError, "missing")

	_ = KindNotFoundError.CustomizeStatusCode(http.StatusBadGateway)
	_ = registered.CustomizeStatusCode(599)

	if GetByCode(KindNotFoundError.Code) != KindNotFoundError || Get(KindNotFoundError.Name) != KindNotFoundError {
		t.Error("a customization must not replace the registered kind")
	}
	if got := KindNotFoundError.StatusCode(); got != http.StatusNotFound {
		t.Errorf("the registered kind reports %d, want 404", got)
	}
	if got := template.New().StatusCode(); got != http.StatusNotFound {
		t.Errorf("the template reports %d, want 404", got)
	}
}

func TestCustomizeStatusCode_concurrentUsesDoNotInterfere(t *testing.T) {
	var wg sync.WaitGroup

	for _, code := range []int{http.StatusTeapot, 499, http.StatusBadGateway, 599} {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 500 {
				if got := KindNotFoundError.CustomizeStatusCode(code).StatusCode(); got != code {
					t.Errorf("StatusCode() = %d, want %d", got, code)
					return
				}
			}
		}()
	}

	wg.Wait()

	if got := KindNotFoundError.StatusCode(); got != http.StatusNotFound {
		t.Errorf("the shared kind reports %d, want 404", got)
	}
}
