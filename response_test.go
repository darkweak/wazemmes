package wazemmes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseUnmarshalHeaders(t *testing.T) {
	cases := map[string]struct {
		input string
		want  http.Header
	}{
		"php empty array": {`{"headers":[],"body":"b","status":201}`, nil},
		"null":            {`{"headers":null,"body":"b","status":201}`, nil},
		"missing":         {`{"body":"b","status":201}`, nil},
		"list values":     {`{"headers":{"X-A":["1","2"]},"body":"b","status":201}`, http.Header{"X-A": {"1", "2"}}},
		"string value":    {`{"headers":{"X-A":"1"},"body":"b","status":201}`, http.Header{"X-A": {"1"}}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var r response
			if err := json.Unmarshal([]byte(tc.input), &r); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if r.Body != "b" || r.Status != 201 {
				t.Fatalf("unexpected body/status: %q %d", r.Body, r.Status)
			}

			if len(r.Headers) != len(tc.want) {
				t.Fatalf("headers = %v, want %v", r.Headers, tc.want)
			}

			for k, v := range tc.want {
				if got := r.Headers[k]; len(got) != len(v) || got[0] != v[0] {
					t.Fatalf("header %s = %v, want %v", k, got, v)
				}
			}
		})
	}

	var r response
	if err := json.Unmarshal([]byte(`{"headers":{"X-A":42}}`), &r); err == nil {
		t.Fatal("expected an error for a non-string header value")
	}
}

func TestWriteResponseStatusAndHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	w := BuildWriter(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	err := writeResponse(w, &baseHandler{Response: response{
		Headers: http.Header{"x-custom": {"v"}},
		Body:    "created",
		Status:  http.StatusCreated,
	}})
	if err != nil {
		t.Fatal(err)
	}

	w.Flush()

	if rec.Code != http.StatusCreated || rec.Body.String() != "created" || rec.Header().Get("X-Custom") != "v" {
		t.Fatalf("got %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}

	if err := writeResponse(w, &baseHandler{Response: response{Status: 42}}); err == nil {
		t.Fatal("expected an error for an invalid status code")
	}
}
