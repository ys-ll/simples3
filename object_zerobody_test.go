package simples3

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A zero-byte upload (empty file, directory marker) must succeed: FilePut
// used to fail with io.EOF because it read the body into a zero-length
// buffer before sending the request.
func TestS3_FilePut_EmptyBody(t *testing.T) {
	var gotBody []byte
	var gotContentLength string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentLength = r.Header.Get("Content-Length")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("ETag", `"d41d8cd98f00b204e9800998ecf8427e"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s3 := New("us-east-1", "test-access", "test-secret")
	s3.SetEndpoint(srv.URL)

	resp, err := s3.FilePut(UploadInput{
		Bucket:      "test-bucket",
		ObjectKey:   "empty-dir/",
		ContentType: "application/x-directory",
		Body:        bytes.NewReader(nil),
	})
	if err != nil {
		t.Fatalf("S3.FilePut() with empty body returned error: %v", err)
	}
	if resp.ETag == "" {
		t.Errorf("S3.FilePut() returned empty response: %+v", resp)
	}
	if gotContentLength != "0" {
		t.Errorf("Content-Length = %q, want %q", gotContentLength, "0")
	}
	if len(gotBody) != 0 {
		t.Errorf("request body = %q, want empty", gotBody)
	}
}
