// Copyright © 2020 Siftrics
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/siftrics/hydra"
)

func main() {
	containsHelp := false
	for _, s := range os.Args[1:] {
		if s == "-h" || s == "--help" {
			containsHelp = true
			break
		}
	}
	if len(os.Args) == 1 || containsHelp {
		fmt.Fprintf(os.Stderr, `usage: ./hydra <--prompt-api-key|--api-key-file filename> <-d|--data-source-id> <image/document, ...>

examples:
 ./hydra -d my-data-source receipt_1.jpg receipt_2.pdf --prompt-api-key invoice.png
 ./hydra -d my-data-source invoice.pdf receipt.png --api-key-file my_api_key.txt

optional flags:
 [-o|--output-file]          output file path.
 [-f|--do-faster]            process files in half the time at the risk of inaccurate results.
                             --do-faster typically succeeds when files are rotated less than 90 degrees.
 [-j|--return-jpgs]          if data source returns cropped images, return in JPG format (PNG format is default).
 [-q|--jpg-quality <1-100>]  JPG quality. Number between 1 and 100 inclusive. Default 85.
`)
		os.Exit(1)
	}

	cfg := hydra.Config{
		DoFaster: false,
	}
	promptApiKey := false
	apiKeyFile := ""
	dataSourceId := ""
	outputFile := ""
	inputFiles := make([]string, 0)
	for i, s := range os.Args {
		if i == 0 {
			continue
		}
		switch s {
		case "--prompt-api-key":
			promptApiKey = true
		case "--api-key-file":
			if promptApiKey {
				fmt.Fprintf(os.Stderr, `error: Both --prompt-api-key and --api-key-file were specified.
This does not make sense, since each flag is used to pass in an API key but the program does not require two API keys.
Run ./hydra -h for more help.
`)
				os.Exit(1)
			}
			if i+1 >= len(os.Args) {
				fmt.Fprintf(os.Stderr, `error: --api-key-file was specified but no filename came after it.
--api-key-file is supposed to be followed by the name of a file which contains an API key on a single line of text.
Run ./hydra -h for more help.
`)
				os.Exit(1)
			}
			if apiKeyFile != "" {
				fmt.Fprintf(os.Stderr, `error: --api-key-file was specified twice but it should only be specified once.
Run ./hydra -h for more help.
`)
				os.Exit(1)
			}
			apiKeyFile = os.Args[i+1]
		case "-o":
			fallthrough
		case "--output":
			if i+1 >= len(os.Args) {
				fmt.Fprintf(os.Stderr, `error: -o (or --output) was specified but no filename came after it.
--output is supposed to be followed by the name of the file which will contain the recognized text from your images/documents.
Run ./hydra -h for more help.
`)
				os.Exit(1)
			}
			if outputFile != "" {
				fmt.Fprintf(os.Stderr, `error: -o (or --output) was specified twice but it should only be specified once.
Run ./hydra -h for more help.
`)
				os.Exit(1)
			}
			outputFile = os.Args[i+1]
		case "-d":
			fallthrough
		case "--data-source-id":
			if i+1 >= len(os.Args) {
				fmt.Fprintf(os.Stderr, `error: -d (or --data-source-id) was specified but no data source id came after it.
--data-source-id is supposed to be followed by the name of the data source id associated to the images/documents you are uploading.
Run ./hydra -h for more help.
`)
				os.Exit(1)
			}
			if dataSourceId != "" {
				fmt.Fprintf(os.Stderr, `error: -d (or --data-source-id) was specified twice but it should only be specified once.
Run ./hydra -h for more help.
`)
				os.Exit(1)
			}
			dataSourceId = os.Args[i+1]
		case "-f":
			fallthrough
		case "--do-faster":
			cfg.DoFaster = true
		case "-j":
			fallthrough
		case "--return-jpgs":
			cfg.ReturnJpgs = true
		case "-q":
			fallthrough
		case "--jpg-quality":
			if i+1 >= len(os.Args) {
				fmt.Fprintf(os.Stderr, `error: -q (or --jpg-quality) was specified but no number between 1 and 100 inclusive came after it.
--jpg-quality is supposed to be followed by a number between 1 and 100 inclusive.
Run ./hydra -h for more help.
`)
				os.Exit(1)
			}
			if cfg.JpgQuality != 0 {
				fmt.Fprintf(os.Stderr, `error: -j (or --jpg-quality) was specified twice but it should only be specified once.
Run ./hydra -h for more help.
`)
				os.Exit(1)
			}
			jpgQuality, err := strconv.Atoi(os.Args[i+1])
			if err != nil || jpgQuality < 1 || jpgQuality > 100 {
				fmt.Fprintf(os.Stderr, `error: --jpg-quality is supposed to be followed by a number between 1 and 100 inclusive.
Run ./hydra -h for more help.
`)
				os.Exit(1)
			}
			cfg.JpgQuality = jpgQuality
		default:
			if !(os.Args[i-1] == "--api-key-file" ||
				os.Args[i-1] == "-o" || os.Args[i-1] == "--output" ||
				os.Args[i-1] == "-d" || os.Args[i-1] == "--data-source-id" ||
				os.Args[i-1] == "-q" || os.Args[i-1] == "--jpg-quality") {
				inputFiles = append(inputFiles, s)
			}
		}
	}
	if dataSourceId == "" {
		fmt.Fprintf(os.Stderr, `error: You must specify --data-source-id (you can use -d for shorthand).
Run ./sight -h for more help.
`)
		os.Exit(1)
	}
	if cfg.JpgQuality != 0 && !cfg.ReturnJpgs {
		fmt.Fprintf(os.Stderr, `error: You must specify --return-jpgs (-j) if you use --jpg-quality (-q).
Run ./sight -h for more help.
`)
		os.Exit(1)
	}
	var of io.Writer
	if outputFile == "" {
		of = os.Stdout
	} else {
		var err error
		of, err = os.Create(outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
	if len(inputFiles) == 0 {
		fmt.Fprintf(os.Stderr, `error: You must specify documents or images in which to recognize text.
Run ./hydra -h for more help.
`)
		os.Exit(1)
	}

	var client *hydra.Client
	var apiKeyBytes []byte
	var err error
	if promptApiKey {
		fmt.Print("enter your Hydra API key: ")
		apiKeyBytes, err = terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to read api key from stdin: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("")
	} else {
		if apiKeyFile == "" {
			fmt.Fprintf(os.Stderr, `error: You must specify either --prompt-api-key or --api-key-file <filename>.
Run ./hydra -h for more help.
`)
			os.Exit(1)
		}
		apiKeyBytes, err = ioutil.ReadFile(apiKeyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
	apiKey := strings.TrimSpace(string(apiKeyBytes))
	if len(apiKey) != len("xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx") {
		fmt.Fprintf(os.Stderr, "error: the provided API key is not valid\nAPI keys should look like xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx\n")
		if apiKeyFile != "" {
			fmt.Fprintf(os.Stderr, "you specified to read the API key from the file %v\n", apiKeyFile)
		}
		fmt.Fprintf(os.Stderr, "run ./hydra --help to see how to provide an API key\n")
		os.Exit(1)
	}
	client = hydra.NewClient(apiKey)
	if outputFile != "" {
		fmt.Println("Uploading files...")
	}

	filesChan, err := client.RecognizeCfg(cfg, dataSourceId, inputFiles...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(of, `{"Rows":[`)
	isFirstFile := true
	numFilesComplete := 0
	for {
		rf, isOpen := <-filesChan
		if !isOpen {
			break
		}
		if !isFirstFile {
			fmt.Fprintf(of, ",")
		} else {
			isFirstFile = false
		}
		jsonBytes, err := json.Marshal(rf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nerror: failed to serialize JSON: %v\n", err)
			os.Exit(1)
		}
		of.Write(jsonBytes)

		numFilesComplete++
		if outputFile != "" {
			fmt.Printf("%v out of %v input files are complete\n", numFilesComplete, len(inputFiles))
		}
	}
	fmt.Fprintf(of, "]}\n")
}
