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

package output

import (
	"bufio"
	"fmt"
	"html"
	"os"
	"strings"

	fileUtils "github.com/edoardottt/cariddi/internal/file"
	"github.com/edoardottt/cariddi/pkg/input"
	"github.com/edoardottt/cariddi/pkg/scanner"
)

type ResultStream interface {
	URLCount() int
	SecretCount() int
	EndpointCount() int
	ExtensionCount() int
	ErrorCount() int
	InfoCount() int
	ForEachURL(func(string) error) error
	ForEachSecret(func(scanner.SecretMatched) error) error
	ForEachEndpoint(func(scanner.EndpointMatched) error) error
	ForEachExtension(func(scanner.FileTypeMatched) error) error
	ForEachError(func(scanner.ErrorMatched) error) error
	ForEachInfo(func(scanner.InfoMatched) error) error
}

// TxtOutputStream writes text output without materializing all results in memory.
func TxtOutputStream(flags input.Input, results ResultStream) error {
	exists, err := fileUtils.ElementExists(CariddiOutputFolder)
	if err != nil {
		return err
	}

	if !exists {
		fileUtils.CreateOutputFolder()
	}

	resultFilename, err := fileUtils.CreateOutputFileE(flags.TXTout, ResultFile, "txt")
	if err != nil {
		return err
	}
	if err := writeTxtRows(resultFilename, func(write func(string) error) error {
		return results.ForEachURL(write)
	}); err != nil {
		return err
	}

	if flags.Secrets && results.SecretCount() > 0 {
		secretFilename, err := fileUtils.CreateOutputFileE(flags.TXTout, SecretsFile, "txt")
		if err != nil {
			return err
		}
		if err := writeTxtRows(secretFilename, func(write func(string) error) error {
			return results.ForEachSecret(func(elem scanner.SecretMatched) error {
				return write(fmt.Sprintf("%s - %s in %s", elem.Secret.Name, elem.Match, elem.URL))
			})
		}); err != nil {
			return err
		}
	}

	if flags.Endpoints && results.EndpointCount() > 0 {
		endpointFilename, err := fileUtils.CreateOutputFileE(flags.TXTout, EndpointsFile, "txt")
		if err != nil {
			return err
		}
		if err := writeTxtRows(endpointFilename, func(write func(string) error) error {
			return results.ForEachEndpoint(func(elem scanner.EndpointMatched) error {
				for _, parameter := range elem.Parameters {
					if err := write(fmt.Sprintf("%s in %s", formatParameter(parameter), elem.URL)); err != nil {
						return err
					}
				}

				return nil
			})
		}); err != nil {
			return err
		}
	}

	if hasValidExtensionFlag(flags.Extensions) && results.ExtensionCount() > 0 {
		extensionsFilename, err := fileUtils.CreateOutputFileE(flags.TXTout, ExtensionsFile, "txt")
		if err != nil {
			return err
		}
		if err := writeTxtRows(extensionsFilename, func(write func(string) error) error {
			return results.ForEachExtension(func(elem scanner.FileTypeMatched) error {
				return write(fmt.Sprintf("%s in %s", elem.Filetype.Extension, elem.URL))
			})
		}); err != nil {
			return err
		}
	}

	if flags.Errors && results.ErrorCount() > 0 {
		errorsFilename, err := fileUtils.CreateOutputFileE(flags.TXTout, ErrorsFile, "txt")
		if err != nil {
			return err
		}
		if err := writeTxtRows(errorsFilename, func(write func(string) error) error {
			return results.ForEachError(func(elem scanner.ErrorMatched) error {
				return write(fmt.Sprintf("%s - %s in %s", elem.Error.ErrorName, elem.Match, elem.URL))
			})
		}); err != nil {
			return err
		}
	}

	if flags.Info && results.InfoCount() > 0 {
		infosFilename, err := fileUtils.CreateOutputFileE(flags.TXTout, InfosFile, "txt")
		if err != nil {
			return err
		}
		if err := writeTxtRows(infosFilename, func(write func(string) error) error {
			return results.ForEachInfo(func(elem scanner.InfoMatched) error {
				return write(fmt.Sprintf("%s - %s in %s", elem.Info.Name, elem.Match, elem.URL))
			})
		}); err != nil {
			return err
		}
	}

	return nil
}

// HTMLOutputStream writes HTML output without materializing all results in memory.
func HTMLOutputStream(flags input.Input, resultFilename string, results ResultStream) error {
	exists, err := fileUtils.ElementExists(CariddiOutputFolder)
	if err != nil {
		return err
	}

	if !exists {
		fileUtils.CreateOutputFolder()
	}

	if err := headerHTMLStream("Results found", resultFilename); err != nil {
		return err
	}
	if err := results.ForEachURL(func(elem string) error {
		return appendOutputToHTMLStream(elem, "", resultFilename, true)
	}); err != nil {
		return err
	}
	if err := footerHTMLStream(resultFilename); err != nil {
		return err
	}

	if flags.Secrets && results.SecretCount() > 0 {
		if err := headerHTMLStream("Secrets found", resultFilename); err != nil {
			return err
		}
		if err := results.ForEachSecret(func(elem scanner.SecretMatched) error {
			return appendOutputToHTMLStream(fmt.Sprintf("%s - %s in %s", elem.Secret.Name, elem.Match, elem.URL),
				"", resultFilename, false)
		}); err != nil {
			return err
		}
		if err := footerHTMLStream(resultFilename); err != nil {
			return err
		}
	}

	if flags.Endpoints && results.EndpointCount() > 0 {
		if err := headerHTMLStream("Endpoints found", resultFilename); err != nil {
			return err
		}
		if err := results.ForEachEndpoint(func(elem scanner.EndpointMatched) error {
			for _, parameter := range elem.Parameters {
				if err := appendOutputToHTMLStream(fmt.Sprintf("%s in %s", formatParameter(parameter), elem.URL),
					"", resultFilename, false); err != nil {
					return err
				}
			}

			return nil
		}); err != nil {
			return err
		}
		if err := footerHTMLStream(resultFilename); err != nil {
			return err
		}
	}

	if hasValidExtensionFlag(flags.Extensions) && results.ExtensionCount() > 0 {
		if err := headerHTMLStream("Extensions found", resultFilename); err != nil {
			return err
		}
		if err := results.ForEachExtension(func(elem scanner.FileTypeMatched) error {
			return appendOutputToHTMLStream(fmt.Sprintf("%s in %s", elem.Filetype.Extension, elem.URL),
				"", resultFilename, false)
		}); err != nil {
			return err
		}
		if err := footerHTMLStream(resultFilename); err != nil {
			return err
		}
	}

	if flags.Errors && results.ErrorCount() > 0 {
		if err := headerHTMLStream("Errors found", resultFilename); err != nil {
			return err
		}
		if err := results.ForEachError(func(elem scanner.ErrorMatched) error {
			return appendOutputToHTMLStream(fmt.Sprintf("%s - %s in %s", elem.Error.ErrorName, elem.Match, elem.URL),
				"", resultFilename, false)
		}); err != nil {
			return err
		}
		if err := footerHTMLStream(resultFilename); err != nil {
			return err
		}
	}

	if flags.Info && results.InfoCount() > 0 {
		if err := headerHTMLStream("Useful informations found", resultFilename); err != nil {
			return err
		}
		if err := results.ForEachInfo(func(elem scanner.InfoMatched) error {
			return appendOutputToHTMLStream(elem.Info.Name+" - "+html.EscapeString(elem.Match)+" in "+elem.URL,
				"", resultFilename, false)
		}); err != nil {
			return err
		}
		if err := footerHTMLStream(resultFilename); err != nil {
			return err
		}
	}

	return bannerFooterHTMLStream(resultFilename)
}

func PrintFindingsStream(flags input.Input, results ResultStream) error {
	if flags.JSON || flags.Plain {
		return nil
	}

	if results.SecretCount() > 0 {
		if err := results.ForEachSecret(func(elem scanner.SecretMatched) error {
			EncapsulateCustomGreen(elem.Secret.Name, fmt.Sprintf("%s in %s", elem.Match, elem.URL))
			return nil
		}); err != nil {
			return err
		}
	}

	if results.EndpointCount() > 0 {
		if err := results.ForEachEndpoint(func(elem scanner.EndpointMatched) error {
			for _, parameter := range elem.Parameters {
				EncapsulateCustomGreen(formatParameter(parameter), fmt.Sprintf(" in %s", elem.URL))
			}

			return nil
		}); err != nil {
			return err
		}
	}

	if results.ExtensionCount() > 0 {
		if err := results.ForEachExtension(func(elem scanner.FileTypeMatched) error {
			EncapsulateCustomGreen(elem.Filetype.Extension, fmt.Sprintf("%s matched!", elem.URL))
			return nil
		}); err != nil {
			return err
		}
	}

	if results.ErrorCount() > 0 {
		if err := results.ForEachError(func(elem scanner.ErrorMatched) error {
			EncapsulateCustomGreen(elem.Error.ErrorName, fmt.Sprintf("%s in %s", elem.Match, elem.URL))
			return nil
		}); err != nil {
			return err
		}
	}

	if results.InfoCount() > 0 {
		if err := results.ForEachInfo(func(elem scanner.InfoMatched) error {
			EncapsulateCustomGreen(elem.Info.Name, fmt.Sprintf("%s in %s", elem.Match, elem.URL))
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func writeTxtRows(filename string, iterate func(func(string) error) error) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, fileUtils.Permission0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	if err := iterate(func(line string) error {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	return writer.Flush()
}

func formatParameter(parameter scanner.Parameter) string {
	var sb strings.Builder
	sb.WriteString(parameter.Parameter)

	if len(parameter.Attacks) > 0 {
		sb.WriteString(" -")
		for _, attack := range parameter.Attacks {
			sb.WriteString(" ")
			sb.WriteString(attack)
		}
	}

	return sb.String()
}

func appendOutputToHTMLStream(output string, status string, filename string, isLink bool) error {
	if isLink {
		statusColor := status
		if status != "" {
			if string(status[0]) == "2" || string(status[0]) == "3" {
				statusColor = "<p style='color:green;display:inline'>" + html.EscapeString(status) + "</p>"
			} else {
				statusColor = "<p style='color:red;display:inline'>" + html.EscapeString(status) + "</p>"
			}
		}

		return appendHTMLStream(filename, "<li><a target='_blank' href='"+
			html.EscapeString(output)+"'>"+
			html.EscapeString(output)+
			"</a> "+html.EscapeString(statusColor)+"</li>\n")
	}

	return appendHTMLStream(filename, "<li>"+html.EscapeString(output)+"</li>\n")
}

func headerHTMLStream(header string, filename string) error {
	return appendHTMLStream(filename, "<h3>"+html.EscapeString(header)+"</h3><ul>\n")
}

func footerHTMLStream(filename string) error {
	return appendHTMLStream(filename, "</ul>\n")
}

func bannerFooterHTMLStream(filename string) error {
	return appendHTMLStream(filename, HTMLBannerFooter)
}

func appendHTMLStream(filename string, content string) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, fileUtils.Permission0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)

	return err
}
