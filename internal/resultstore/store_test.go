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

package resultstore_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/edoardottt/cariddi/internal/resultstore"
	"github.com/edoardottt/cariddi/pkg/scanner"
)

func TestStoreDeduplicatesLargeURLSet(t *testing.T) {
	store := newTestStore(t)
	defer closeTestStore(t, store)

	for i := range 10000 {
		store.AddURL(fmt.Sprintf("https://example.com/%d", i%257))
	}

	if got, want := store.URLCount(), 257; got != want {
		t.Fatalf("URLCount() = %d, want %d", got, want)
	}

	var urls []string
	if err := store.ForEachURL(func(input string) error {
		urls = append(urls, input)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if len(urls) != 257 {
		t.Fatalf("streamed URLs = %d, want 257", len(urls))
	}

	for i, input := range urls {
		want := fmt.Sprintf("https://example.com/%d", i)
		if input != want {
			t.Fatalf("urls[%d] = %q, want %q", i, input, want)
		}
	}
}

func TestStoreMatchesExistingDedupeKeys(t *testing.T) {
	store := newTestStore(t)
	defer closeTestStore(t, store)

	store.AddSecrets(
		scanner.SecretMatched{Secret: scanner.Secret{Name: "first"}, URL: "https://a.example", Match: "same-secret"},
		scanner.SecretMatched{Secret: scanner.Secret{Name: "second"}, URL: "https://b.example", Match: "same-secret"},
	)
	store.AddEndpoints(
		scanner.EndpointMatched{URL: "https://a.example?id=1", Parameters: []scanner.Parameter{{Parameter: "id"}}},
		scanner.EndpointMatched{URL: "https://a.example?id=1", Parameters: []scanner.Parameter{{Parameter: "token"}}},
	)
	store.AddExtensions(
		scanner.FileTypeMatched{Filetype: scanner.FileType{Extension: "pdf", Severity: 7}, URL: "https://a.example/a.pdf"},
		scanner.FileTypeMatched{Filetype: scanner.FileType{Extension: "doc", Severity: 5}, URL: "https://a.example/a.pdf"},
	)
	store.AddErrors(
		scanner.ErrorMatched{Error: scanner.Error{ErrorName: "sql"}, URL: "https://a.example", Match: "same-error"},
		scanner.ErrorMatched{Error: scanner.Error{ErrorName: "sql"}, URL: "https://b.example", Match: "same-error"},
	)
	store.AddInfos(
		scanner.InfoMatched{Info: scanner.Info{Name: "first"}, URL: "https://a.example", Match: "same-info"},
		scanner.InfoMatched{Info: scanner.Info{Name: "second"}, URL: "https://b.example", Match: "same-info"},
	)

	if got, want := store.SecretCount(), 1; got != want {
		t.Fatalf("SecretCount() = %d, want %d", got, want)
	}
	if got, want := store.EndpointCount(), 1; got != want {
		t.Fatalf("EndpointCount() = %d, want %d", got, want)
	}
	if got, want := store.ExtensionCount(), 1; got != want {
		t.Fatalf("ExtensionCount() = %d, want %d", got, want)
	}
	if got, want := store.ErrorCount(), 2; got != want {
		t.Fatalf("ErrorCount() = %d, want %d", got, want)
	}
	if got, want := store.InfoCount(), 1; got != want {
		t.Fatalf("InfoCount() = %d, want %d", got, want)
	}

	var secrets []scanner.SecretMatched
	if err := store.ForEachSecret(func(item scanner.SecretMatched) error {
		secrets = append(secrets, item)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := secrets[0].URL, "https://a.example"; got != want {
		t.Fatalf("first secret URL = %q, want %q", got, want)
	}
}

func TestStoreConcurrentAdds(t *testing.T) {
	store := newTestStore(t)
	defer closeTestStore(t, store)

	var wg sync.WaitGroup
	for worker := range 32 {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()

			for i := range 1000 {
				store.AddURL(fmt.Sprintf("https://example.com/%d", i%100))
				store.AddErrors(scanner.ErrorMatched{
					Error: scanner.Error{ErrorName: "err"},
					URL:   fmt.Sprintf("https://example.com/%d", worker%4),
					Match: fmt.Sprintf("match-%d", i%10),
				})
			}
		}(worker)
	}

	wg.Wait()

	if got, want := store.URLCount(), 100; got != want {
		t.Fatalf("URLCount() = %d, want %d", got, want)
	}
	if got, want := store.ErrorCount(), 40; got != want {
		t.Fatalf("ErrorCount() = %d, want %d", got, want)
	}
}

func TestStoreCloseIsIdempotent(t *testing.T) {
	store := newTestStore(t)
	tempDir := store.TempDir()
	if !strings.Contains(filepath.Base(tempDir), resultstore.TempDirPrefix) {
		t.Fatalf("TempDir() = %q, want prefix %q", tempDir, resultstore.TempDirPrefix)
	}

	store.AddURL("https://example.com")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("temp dir still exists after Close(): %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupStaleRemovesOldResultTempsOnly(t *testing.T) {
	parent := t.TempDir()
	now := time.Now()
	oldTime := now.Add(-(resultstore.DefaultStaleAge + time.Hour))
	freshTime := now.Add(-time.Hour)

	oldDir := filepath.Join(parent, resultstore.TempDirPrefix+"old")
	freshDir := filepath.Join(parent, resultstore.TempDirPrefix+"fresh")
	unrelatedDir := filepath.Join(parent, "other-old")
	legacyFile := filepath.Join(parent, "cariddi-results-old.jsonl")

	for _, dir := range []string{oldDir, freshDir, unrelatedDir} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(legacyFile, []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{oldDir, unrelatedDir, legacyFile} {
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(freshDir, freshTime, freshTime); err != nil {
		t.Fatal(err)
	}

	if err := resultstore.CleanupStaleIn(parent, resultstore.DefaultStaleAge); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{oldDir, legacyFile} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("stale temp path still exists %q: %v", path, err)
		}
	}
	for _, path := range []string{freshDir, unrelatedDir} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("non-stale path removed %q: %v", path, err)
		}
	}
}

func TestStoreStreamsScannerValues(t *testing.T) {
	store := newTestStore(t)
	defer closeTestStore(t, store)

	store.AddEndpoints(scanner.EndpointMatched{
		URL: "https://example.com?id=1",
		Parameters: []scanner.Parameter{
			{Parameter: "id", Attacks: []string{"XSS", "SQLi"}},
		},
	})
	store.AddExtensions(scanner.FileTypeMatched{
		Filetype: scanner.FileType{Extension: "pdf", Severity: 7},
		URL:      "https://example.com/file.pdf",
	})

	var endpoints []scanner.EndpointMatched
	if err := store.ForEachEndpoint(func(item scanner.EndpointMatched) error {
		endpoints = append(endpoints, item)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	wantEndpoints := []scanner.EndpointMatched{{
		URL: "https://example.com?id=1",
		Parameters: []scanner.Parameter{
			{Parameter: "id", Attacks: []string{"XSS", "SQLi"}},
		},
	}}
	if !reflect.DeepEqual(endpoints, wantEndpoints) {
		t.Fatalf("endpoints = %#v, want %#v", endpoints, wantEndpoints)
	}

	var extensions []scanner.FileTypeMatched
	if err := store.ForEachExtension(func(item scanner.FileTypeMatched) error {
		extensions = append(extensions, item)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	wantExtensions := []scanner.FileTypeMatched{{
		Filetype: scanner.FileType{Extension: "pdf", Severity: 7},
		URL:      "https://example.com/file.pdf",
	}}
	if !reflect.DeepEqual(extensions, wantExtensions) {
		t.Fatalf("extensions = %#v, want %#v", extensions, wantExtensions)
	}
}

func newTestStore(t *testing.T) *resultstore.Store {
	t.Helper()

	store, err := resultstore.New()
	if err != nil {
		t.Fatal(err)
	}

	return store
}

func closeTestStore(t *testing.T, store *resultstore.Store) {
	t.Helper()

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
