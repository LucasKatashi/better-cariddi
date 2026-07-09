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

package input_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/edoardottt/cariddi/pkg/input"
)

func TestForEachTargetStreamsLargeUniqueInput(t *testing.T) {
	oldStdin := os.Stdin
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = read
	defer func() {
		os.Stdin = oldStdin
	}()

	go func() {
		defer write.Close()
		for i := range 10000 {
			_, _ = fmt.Fprintf(write, "EXAMPLE-%d.com\n", i%128)
		}
		_, _ = fmt.Fprintln(write, "ab")
	}()

	var got []string
	if err := input.ForEachTarget(func(target string) error {
		got = append(got, target)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if len(got) != 128 {
		t.Fatalf("targets = %d, want 128", len(got))
	}

	for i, target := range got {
		want := fmt.Sprintf("example-%d.com", i)
		if target != want {
			t.Fatalf("target[%d] = %q, want %q", i, target, want)
		}
	}
}

func TestScanTargetsUsesStreamingDedupe(t *testing.T) {
	oldStdin := os.Stdin
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = read
	defer func() {
		os.Stdin = oldStdin
	}()

	go func() {
		defer write.Close()
		_, _ = fmt.Fprintln(write, "One.example")
		_, _ = fmt.Fprintln(write, "one.example")
		_, _ = fmt.Fprintln(write, "Two.example")
	}()

	got := input.ScanTargets()
	want := []string{"one.example", "two.example"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ScanTargets() = %#v, want %#v", got, want)
	}
}
