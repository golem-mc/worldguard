package worldguardplugin

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadPaste(t *testing.T) {
	var got string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/signed_paste_v2" && r.Header.Get("x-paste-meta-from") == "EngineHub":
			fmt.Fprintf(w, `{"viewUrl":"https://view/abc","uploadUrl":%q,"headers":{"x-test":"1"}}`, srv.URL+"/up")
		case r.URL.Path == "/up" && r.Method == http.MethodPut && r.Header.Get("x-test") == "1":
			b, _ := io.ReadAll(r.Body)
			got = string(b)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	old := pasteBase
	pasteBase = srv.URL
	defer func() { pasteBase = old }()

	url, err := uploadPaste("hello")
	if err != nil || url != "https://view/abc" || got != "hello" {
		t.Fatalf("uploadPaste = %q, %v, uploaded %q", url, err, got)
	}
	pasteBase = srv.URL + "/missing"
	if _, err := uploadPaste("x"); err == nil {
		t.Fatal("expected failure on a refused signing request")
	}
}
