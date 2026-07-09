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

package resultstore

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/edoardottt/cariddi/pkg/scanner"
)

type urlRecord struct {
	URL string `json:"url"`
}

type secretRecord struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Match string `json:"match"`
}

type endpointRecord struct {
	URL        string              `json:"url"`
	Parameters []scanner.Parameter `json:"parameters"`
}

type extensionRecord struct {
	Extension string `json:"extension"`
	Severity  int    `json:"severity"`
	URL       string `json:"url"`
}

type errorRecord struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Match string `json:"match"`
}

type infoRecord struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Match string `json:"match"`
}

type recordStore[T any] struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
	seen    map[[sha256.Size]byte]struct{}
	count   int
	err     error
	closed  bool
}

var errStoreClosed = errors.New("result store closed")

func newRecordStore[T any]() (*recordStore[T], error) {
	file, err := os.CreateTemp("", "cariddi-results-*.jsonl")
	if err != nil {
		return nil, err
	}

	return &recordStore[T]{
		file:    file,
		encoder: json.NewEncoder(file),
		seen:    make(map[[sha256.Size]byte]struct{}),
	}, nil
}

func (s *recordStore[T]) Add(key string, record T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return
	}
	if s.closed {
		s.err = errStoreClosed
		return
	}

	digest := sha256.Sum256([]byte(key))
	if _, ok := s.seen[digest]; ok {
		return
	}

	if err := s.encoder.Encode(record); err != nil {
		s.err = err
		return
	}

	s.seen[digest] = struct{}{}
	s.count++
}

func (s *recordStore[T]) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.count
}

func (s *recordStore[T]) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.err
}

func (s *recordStore[T]) ForEach(fn func(T) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}
	if s.closed {
		return errStoreClosed
	}

	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	decoder := json.NewDecoder(s.file)
	for {
		var record T
		if err := decoder.Decode(&record); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return err
		}

		if err := fn(record); err != nil {
			return err
		}
	}
}

func (s *recordStore[T]) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := s.file.Name()
	if s.closed {
		return nil
	}

	closeErr := s.file.Close()
	s.closed = true
	removeErr := os.Remove(name)
	if closeErr != nil {
		return closeErr
	}
	if errors.Is(removeErr, os.ErrNotExist) {
		return nil
	}

	return removeErr
}

// Store keeps fixed-size dedupe digests in memory and streams unique result payloads to temp files.
type Store struct {
	urls       *recordStore[urlRecord]
	secrets    *recordStore[secretRecord]
	endpoints  *recordStore[endpointRecord]
	extensions *recordStore[extensionRecord]
	errors     *recordStore[errorRecord]
	infos      *recordStore[infoRecord]
}

func New() (*Store, error) {
	urls, err := newRecordStore[urlRecord]()
	if err != nil {
		return nil, err
	}

	secrets, err := newRecordStore[secretRecord]()
	if err != nil {
		_ = urls.Close()
		return nil, err
	}

	endpoints, err := newRecordStore[endpointRecord]()
	if err != nil {
		_ = urls.Close()
		_ = secrets.Close()
		return nil, err
	}

	extensions, err := newRecordStore[extensionRecord]()
	if err != nil {
		_ = urls.Close()
		_ = secrets.Close()
		_ = endpoints.Close()
		return nil, err
	}

	errors, err := newRecordStore[errorRecord]()
	if err != nil {
		_ = urls.Close()
		_ = secrets.Close()
		_ = endpoints.Close()
		_ = extensions.Close()
		return nil, err
	}

	infos, err := newRecordStore[infoRecord]()
	if err != nil {
		_ = urls.Close()
		_ = secrets.Close()
		_ = endpoints.Close()
		_ = extensions.Close()
		_ = errors.Close()
		return nil, err
	}

	return &Store{
		urls:       urls,
		secrets:    secrets,
		endpoints:  endpoints,
		extensions: extensions,
		errors:     errors,
		infos:      infos,
	}, nil
}

func (s *Store) AddURL(input string) {
	s.urls.Add(input, urlRecord{URL: input})
}

func (s *Store) AddSecrets(input ...scanner.SecretMatched) {
	for _, item := range input {
		s.secrets.Add(item.Match, secretRecord{
			Name:  item.Secret.Name,
			URL:   item.URL,
			Match: item.Match,
		})
	}
}

func (s *Store) AddEndpoints(input ...scanner.EndpointMatched) {
	for _, item := range input {
		s.endpoints.Add(item.URL, endpointRecord{
			URL:        item.URL,
			Parameters: item.Parameters,
		})
	}
}

func (s *Store) AddExtensions(input ...scanner.FileTypeMatched) {
	for _, item := range input {
		s.extensions.Add(item.URL, extensionRecord{
			Extension: item.Filetype.Extension,
			Severity:  item.Filetype.Severity,
			URL:       item.URL,
		})
	}
}

func (s *Store) AddErrors(input ...scanner.ErrorMatched) {
	for _, item := range input {
		s.errors.Add(item.Match+item.URL, errorRecord{
			Name:  item.Error.ErrorName,
			URL:   item.URL,
			Match: item.Match,
		})
	}
}

func (s *Store) AddInfos(input ...scanner.InfoMatched) {
	for _, item := range input {
		s.infos.Add(item.Match, infoRecord{
			Name:  item.Info.Name,
			URL:   item.URL,
			Match: item.Match,
		})
	}
}

func (s *Store) URLCount() int {
	return s.urls.Count()
}

func (s *Store) SecretCount() int {
	return s.secrets.Count()
}

func (s *Store) EndpointCount() int {
	return s.endpoints.Count()
}

func (s *Store) ExtensionCount() int {
	return s.extensions.Count()
}

func (s *Store) ErrorCount() int {
	return s.errors.Count()
}

func (s *Store) InfoCount() int {
	return s.infos.Count()
}

func (s *Store) ForEachURL(fn func(string) error) error {
	return s.urls.ForEach(func(record urlRecord) error {
		return fn(record.URL)
	})
}

func (s *Store) ForEachSecret(fn func(scanner.SecretMatched) error) error {
	return s.secrets.ForEach(func(record secretRecord) error {
		return fn(scanner.SecretMatched{
			Secret: scanner.Secret{Name: record.Name},
			URL:    record.URL,
			Match:  record.Match,
		})
	})
}

func (s *Store) ForEachEndpoint(fn func(scanner.EndpointMatched) error) error {
	return s.endpoints.ForEach(func(record endpointRecord) error {
		return fn(scanner.EndpointMatched{
			Parameters: record.Parameters,
			URL:        record.URL,
		})
	})
}

func (s *Store) ForEachExtension(fn func(scanner.FileTypeMatched) error) error {
	return s.extensions.ForEach(func(record extensionRecord) error {
		return fn(scanner.FileTypeMatched{
			Filetype: scanner.FileType{
				Extension: record.Extension,
				Severity:  record.Severity,
			},
			URL: record.URL,
		})
	})
}

func (s *Store) ForEachError(fn func(scanner.ErrorMatched) error) error {
	return s.errors.ForEach(func(record errorRecord) error {
		return fn(scanner.ErrorMatched{
			Error: scanner.Error{ErrorName: record.Name},
			URL:   record.URL,
			Match: record.Match,
		})
	})
}

func (s *Store) ForEachInfo(fn func(scanner.InfoMatched) error) error {
	return s.infos.ForEach(func(record infoRecord) error {
		return fn(scanner.InfoMatched{
			Info:  scanner.Info{Name: record.Name},
			URL:   record.URL,
			Match: record.Match,
		})
	})
}

func (s *Store) Err() error {
	stores := []struct {
		name string
		err  error
	}{
		{"urls", s.urls.Err()},
		{"secrets", s.secrets.Err()},
		{"endpoints", s.endpoints.Err()},
		{"extensions", s.extensions.Err()},
		{"errors", s.errors.Err()},
		{"infos", s.infos.Err()},
	}

	for _, store := range stores {
		if store.err != nil {
			return fmt.Errorf("%s result store: %w", store.name, store.err)
		}
	}

	return nil
}

func (s *Store) Close() error {
	var closeErr error
	stores := []struct {
		name  string
		close func() error
	}{
		{"urls", s.urls.Close},
		{"secrets", s.secrets.Close},
		{"endpoints", s.endpoints.Close},
		{"extensions", s.extensions.Close},
		{"errors", s.errors.Close},
		{"infos", s.infos.Close},
	}

	for _, store := range stores {
		if err := store.close(); err != nil && closeErr == nil {
			closeErr = fmt.Errorf("%s result store: %w", store.name, err)
		}
	}

	return closeErr
}
