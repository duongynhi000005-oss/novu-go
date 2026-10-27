package hooks

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"
)

type trackingReadCloser struct {
	*bytes.Reader
	closed bool
}

func newTrackingReadCloser(b []byte) *trackingReadCloser {
	return &trackingReadCloser{Reader: bytes.NewReader(b)}
}

func (t *trackingReadCloser) Read(p []byte) (int, error) {
	if t.closed {
		return 0, errors.New("file already closed")
	}
	return t.Reader.Read(p)
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
}

func TestAfterSuccess_PreservesReadableBodyOnEmptyJSONResponse(t *testing.T) {
	h := &NovuHooks{}
	res := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       newTrackingReadCloser(nil),
	}

	out, err := h.AfterSuccess(AfterSuccessContext{}, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatalf("expected readable body after AfterSuccess on empty JSON response, got error: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("expected empty body, got %q", string(raw))
	}
}

func TestAfterSuccess_Skips4xxAnd5xxErrorResponses(t *testing.T) {
	h := &NovuHooks{}
	errPayload := `{"statusCode":404,"message":"Workflow not found","path":"/v2/workflows/non-existent"}`
	origBody := newTrackingReadCloser([]byte(errPayload))
	res := &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       origBody,
	}

	out, err := h.AfterSuccess(AfterSuccessContext{}, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if origBody.closed {
		t.Fatalf("expected 404 response body not to be closed by AfterSuccess")
	}

	raw, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatalf("expected 404 response body to be readable, got: %v", err)
	}
	if string(raw) != errPayload {
		t.Fatalf("expected %s, got %s", errPayload, string(raw))
	}
}

func TestAfterSuccess_UnwrapsSingleDataFieldOn2xx(t *testing.T) {
	h := &NovuHooks{}
	payload := `{"data":{"id":"wf_123","name":"Welcome"}}`
	res := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       newTrackingReadCloser([]byte(payload)),
	}

	out, err := h.AfterSuccess(AfterSuccessContext{}, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	expected := `{"id":"wf_123","name":"Welcome"}`
	if string(raw) != expected {
		t.Fatalf("expected %s, got %s", expected, string(raw))
	}
}
