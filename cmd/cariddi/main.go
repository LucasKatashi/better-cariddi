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

package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	fileUtils "github.com/edoardottt/cariddi/internal/file"
	"github.com/edoardottt/cariddi/internal/resultstore"
	"github.com/edoardottt/cariddi/pkg/crawler"
	"github.com/edoardottt/cariddi/pkg/input"
	"github.com/edoardottt/cariddi/pkg/output"
)

func main() {
	// Scan flags.
	flags := input.ScanFlag()

	// Print version and exit.
	if flags.Version {
		output.Banner()
		os.Exit(0)
	}

	// Print help and exit.
	if flags.Help {
		output.PrintHelp()
		os.Exit(0)
	}

	// Print examples and exit.
	if flags.Examples {
		output.PrintExamples()
		os.Exit(0)
	}

	// If it's possible print the cariddi banner.
	if !flags.Plain {
		output.Banner()
	}

	// Setup the config according to the flags that were
	// passed via the CLI
	config := &crawler.Scan{
		Delay:            flags.Delay,
		Concurrency:      flags.Concurrency,
		Ignore:           flags.Ignore,
		IgnoreTxt:        flags.IgnoreTXT,
		Cache:            flags.Cache,
		JSON:             flags.JSON,
		Timeout:          flags.Timeout,
		Intensive:        flags.Intensive,
		Rua:              flags.Rua,
		Proxy:            flags.Proxy,
		SecretsFlag:      flags.Secrets,
		Plain:            flags.Plain,
		EndpointsFlag:    flags.Endpoints,
		FileType:         flags.Extensions,
		ErrorsFlag:       flags.Errors,
		InfoFlag:         flags.Info,
		Debug:            flags.Debug,
		UserAgent:        flags.UserAgent,
		StoreResp:        flags.StoreResp,
		MaxDepth:         flags.MaxDepth,
		IgnoreExtensions: flags.IgnoreExtensions,
	}

	if err := input.CheckStdin(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Check if there are errors in the flags definition.
	input.CheckFlags(flags)

	// If it is needed, read custom endpoints definition
	// from the specified file.
	if flags.EndpointsFile != "" {
		config.EndpointsSlice = fileUtils.ReadFile(flags.EndpointsFile)
	}

	// If it is needed, read custom secrets definition
	// from the specified file.
	if flags.SecretsFile != "" {
		config.SecretsSlice = fileUtils.ReadFile(flags.SecretsFile)
	}

	// Create output files if needed (txt / html).
	config.Txt = ""
	if flags.TXTout != "" {
		config.Txt = fileUtils.CreateOutputFile(flags.TXTout, "results", "txt")
	}

	var ResultHTML = ""
	if flags.HTMLout != "" {
		ResultHTML = fileUtils.CreateOutputFile(flags.HTMLout, "", "html")
		output.BannerHTML(ResultHTML)
	}

	if config.StoreResp {
		fileUtils.CreateIndexOutputFile("index.responses.txt")
	}

	// Read headers if needed
	if flags.HeadersFile != "" || flags.Headers != "" {
		var headersInput string
		if flags.HeadersFile != "" {
			headersInput = string(fileUtils.ReadEntireFile(flags.HeadersFile))
		} else {
			headersInput = flags.Headers
		}

		config.Headers = input.GetHeaders(headersInput)
	}
	if err := resultstore.CleanupStale(resultstore.DefaultStaleAge); err != nil && flags.Debug {
		fmt.Fprintln(os.Stderr, err)
	}

	results, err := resultstore.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var cleanupOnce sync.Once
	var cleanupErr error
	cleanup := func() error {
		cleanupOnce.Do(func() {
			cleanupErr = results.Close()
		})

		return cleanupErr
	}
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()
	exitWithError := func(err error) {
		fmt.Println(err)
		if closeErr := cleanup(); closeErr != nil {
			fmt.Println(closeErr)
		}

		os.Exit(1)
	}

	chanC := make(chan os.Signal, 1)
	signal.Notify(chanC, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(chanC)
	go func() {
		sig := <-chanC
		if flags.Debug {
			fmt.Fprint(os.Stdout, "\r")
			fmt.Printf("%s received: Exiting immediately\n", sig)
		}
		if err := cleanup(); err != nil && flags.Debug {
			fmt.Println(err)
		}

		os.Exit(1)
	}()

	config.ResultSink = results

	// For each target generate a crawler and collect all the results.
	err = input.ForEachTarget(func(target string) error {
		config.Target = target
		crawler.New(config)

		return results.Err()
	})
	if err != nil {
		exitWithError(err)
	}

	if err := results.Err(); err != nil {
		exitWithError(err)
	}

	// IF TXT OUTPUT >
	if flags.TXTout != "" {
		if err := output.TxtOutputStream(flags, results); err != nil {
			exitWithError(err)
		}
	}

	// IF HTML OUTPUT >
	if flags.HTMLout != "" {
		if err := output.WriteSummaryCardStream(ResultHTML, results.URLCount(), results.SecretCount(), results.EndpointCount(),
			results.ExtensionCount(), results.ErrorCount(), results.InfoCount()); err != nil {
			exitWithError(err)
		}
		if err := output.HTMLOutputStream(flags, ResultHTML, results); err != nil {
			exitWithError(err)
		}
	}

	if err := output.PrintFindingsStream(flags, results); err != nil {
		exitWithError(err)
	}
}
