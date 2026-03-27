package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateSourceURLDetail(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ok, code, msg := ValidateSourceURLDetail(srv.URL)
	if !ok || code != 200 || msg != "HTTP 200 OK" {
		t.Fatalf("expected 200 OK, got ok=%v code=%d msg=%q", ok, code, msg)
	}
}

func TestValidateSourceURLDetailNon200(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	ok, code, msg := ValidateSourceURLDetail(srv.URL)
	if ok || code != 403 {
		t.Fatalf("expected failure with 403, got ok=%v code=%d msg=%q", ok, code, msg)
	}
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
}
