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

package input

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"

	pdutils "github.com/projectdiscovery/utils/file"
)

const (
	coupleSize = 2
)

var ErrNoInput = errors.New("No input provided.")

func CheckStdin() error {
	if !pdutils.HasStdin() {
		return ErrNoInput
	}

	return nil
}

// ScanTargets return the array of elements
// taken as input on stdin.
func ScanTargets() []string {
	var result []string

	if err := ForEachTarget(func(domain string) error {
		result = append(result, domain)
		return nil
	}); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return result
}

// ForEachTarget streams unique targets from stdin.
func ForEachTarget(fn func(string) error) error {
	if err := CheckStdin(); err != nil {
		return err
	}

	seen := make(map[[sha256.Size]byte]struct{})

	// accept domains on stdin
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		domain := strings.ToLower(sc.Text())
		if len(domain) > coupleSize {
			digest := sha256.Sum256([]byte(domain))
			if _, ok := seen[digest]; ok {
				continue
			}

			seen[digest] = struct{}{}
			if err := fn(domain); err != nil {
				return err
			}
		}
	}

	return sc.Err()
}

// GetHeaders returns the headers provided as input
// using the headers flag.
// E.g. -headers \"Cookie: auth=yes;;Client: type=2\".
func GetHeaders(input string) map[string]string {
	result := make(map[string]string)

	if input != "" {
		if !strings.Contains(input, ":") {
			fmt.Println("The headers provided don't contains the : separator.")
			os.Exit(1)
		}

		headers := strings.Split(input, ";;")
		for _, header := range headers {
			var parts []string
			if strings.Contains(header, ":") {
				parts = strings.SplitN(header, ":", coupleSize)
			} else {
				continue
			}

			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	} else {
		fmt.Println("Headers or HeadersFile flag provided, but the content is empty.")
		os.Exit(1)
	}

	if len(result) == 0 {
		fmt.Println("Headers or HeadersFile flag provided, but the content is empty.")
		os.Exit(1)
	}

	return result
}
