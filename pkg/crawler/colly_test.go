/*
==========
Cariddi
==========

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see http://www.gnu.org/licenses/.

	@Repository:  https://github.com/edoardottt/cariddi

	@Author:      edoardottt, https://edoardottt.com

	@License: https://github.com/edoardottt/cariddi/blob/main/LICENSE

*/

package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/edoardottt/cariddi/internal/resultstore"
	"github.com/gocolly/colly/v2"
)

func TestRequestURLString(t *testing.T) {
	u, err := url.Parse("http://example.com/path")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		req  *colly.Request
		want string
		ok   bool
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
			ok:   false,
		},
		{
			name: "nil URL",
			req:  &colly.Request{},
			want: "",
			ok:   false,
		},
		{
			name: "valid URL",
			req:  &colly.Request{URL: u},
			want: "http://example.com/path",
			ok:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := requestURLString(tt.req)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("requestURLString() = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestResponseURLString(t *testing.T) {
	u, err := url.Parse("http://example.com/path")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		resp *colly.Response
		want string
		ok   bool
	}{
		{
			name: "nil response",
			resp: nil,
			want: "",
			ok:   false,
		},
		{
			name: "nil request",
			resp: &colly.Response{},
			want: "",
			ok:   false,
		},
		{
			name: "nil URL",
			resp: &colly.Response{Request: &colly.Request{}},
			want: "",
			ok:   false,
		},
		{
			name: "valid URL",
			resp: &colly.Response{Request: &colly.Request{URL: u}},
			want: "http://example.com/path",
			ok:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := responseURLString(tt.resp)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("responseURLString() = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestNewCollectsURLsUnderConcurrentRequests(t *testing.T) {
	const linkCount = 300

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			var body strings.Builder
			body.WriteString("<html><body>")
			for i := range linkCount {
				body.WriteString(fmt.Sprintf(`<a href="/item-%d?x=%d">link</a>`, i, i))
			}
			body.WriteString("</body></html>")
			_, _ = w.Write([]byte(body.String()))
		default:
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer server.Close()

	results := New(&Scan{
		Target:      server.URL,
		Concurrency: 64,
		Timeout:     5,
		JSON:        true,
	})

	if results == nil {
		t.Fatal("New() returned nil results")
	}

	if got, want := len(results.URLs), linkCount+2; got != want {
		t.Fatalf("len(results.URLs) = %d, want %d", got, want)
	}
}

func TestNewStreamsURLsToResultSink(t *testing.T) {
	const linkCount = 300

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			var body strings.Builder
			body.WriteString("<html><body>")
			for i := range linkCount {
				body.WriteString(fmt.Sprintf(`<a href="/item-%d?x=%d">link</a>`, i, i))
			}
			body.WriteString("</body></html>")
			_, _ = w.Write([]byte(body.String()))
		default:
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer server.Close()

	store, err := resultstore.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	results := New(&Scan{
		Target:      server.URL,
		Concurrency: 64,
		Timeout:     5,
		JSON:        true,
		ResultSink:  store,
	})

	if results == nil {
		t.Fatal("New() returned nil results")
	}

	if got := len(results.URLs); got != 0 {
		t.Fatalf("len(results.URLs) = %d, want 0 when ResultSink is set", got)
	}
	if got, want := store.URLCount(), linkCount+2; got != want {
		t.Fatalf("store.URLCount() = %d, want %d", got, want)
	}
}
