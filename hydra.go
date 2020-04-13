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

package hydra

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// Config is used to consolidate the parameters to the function
// func (c *Client) RecognizeCfg. As the Hydra API becomes more configurable,
// the number of parameters will grow unwieldy. This allows RecognizeCfg
// interface to remain readable (few parameters) and unchanged over time.
type Config struct {
	DoFaster bool
}

type HydraRequest struct {
	Files    []HydraRequestFile `json:"files"`
	DoFaster bool
}

type HydraRequestFile struct {
	MimeType   string
	Base64File string
}

type RecognizedFile struct {
	Error          string
	FileIndex      int
	RecognizedText map[string]interface{}
	Base64Image    string `json:",omitempty"`
}

// Get retrieves the string value associated to the given field.
// If no such field is associated to the given data source,
// or if the field is a table, as opposed to a string, then err != nil.
// The returned string may be an empty string even when err == nil.
func (rf *RecognizedFile) Get(field string) (string, error) {
	v, ok := rf.RecognizedText[field]
	if !ok {
		fields := make([]string, len(rf.RecognizedText), len(rf.RecognizedText))
		i := 0
		for k, _ := range rf.RecognizedText {
			fields[i] = k
			i++
		}
		return "", fmt.Errorf("No such field \"%v\" associated to the given data source. Valid fields: %v", field, fields)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("The field \"%v\" is a table, not a string. Consider using the \"GetTable\" method.", field)
	}
	return s, nil
}

// GetTable retrieves the table associated to the given field.
// If no such field is associated to the given data source,
// or if the field is a string, as opposed to a table, then err != nil.
// The returned table may be empty even when err == nil.
func (rf *RecognizedFile) GetTable(field string) ([]map[string]string, error) {
	v, ok := rf.RecognizedText[field]
	if !ok {
		fields := make([]string, len(rf.RecognizedText), len(rf.RecognizedText))
		i := 0
		for k, _ := range rf.RecognizedText {
			fields[i] = k
			i++
		}
		return nil, fmt.Errorf("No such field \"%v\" associated to the given data source. Valid fields: %v", field, fields)
	}
	interfaces, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("The field \"%v\" is a string, not a table. Consider using the \"Get\" method.", field)
	}
	t := make([]map[string]string, len(interfaces), len(interfaces))
	for i, inter := range interfaces {
		mInter, ok := inter.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("This should never happen. Expected type map[string]interface{}. Got: %T\n", inter)
		}
		m := make(map[string]string)
		for k, sInter := range mInter {
			s, ok := sInter.(string)
			if !ok {
				return nil, fmt.Errorf("This should never happen. Expected type string. Got: %T\n", sInter)
			}
			m[k] = s
		}
		t[i] = m
	}
	return t, nil
}

type RecognizedFiles struct {
	Rows []RecognizedFile
}

type Client struct {
	apiKey string
}

func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey}
}

// Recognize is shorthand for calling RecognizeCfg with all the default config values.
func (c *Client) Recognize(dataSourceId string, filePaths ...string) (<-chan RecognizedFile, error) {
	return c.RecognizeCfg(
		Config{
			DoFaster: false,
		},
		dataSourceId,
		filePaths...,
	)
}

// RecognizeCfg uses the Hydra API to recognize all the text in the given files.
//
// If err != nil, then ioutil.ReadAll failed on a given file, a MIME type was
// failed to be inferred from the suffix (extension) of a given filename, or
// there was an error with the _initial_ HTTP request or response.
//
// This function blocks until receiving a response for the _initial_ HTTP request
// to the Hydra API, so that non-200 responses for the initial request are conveyed
// via the returned error. All remaining work, including any additional network
// requests, is done in a separate goroutine. Accordingly, to avoid the blocking
// nature of the initial network request, this function must be run in a separate
// goroutine.
func (c *Client) RecognizeCfg(cfg Config, dataSourceId string, filePaths ...string) (<-chan RecognizedFile, error) {
	sr := HydraRequest{
		Files:    make([]HydraRequestFile, len(filePaths), len(filePaths)),
		DoFaster: cfg.DoFaster,
	}
	for i, fp := range filePaths {
		if len(fp) < 4 {
			return nil, fmt.Errorf("failed to infer MIME type from file path: %v", fp)
		}
		switch strings.ToLower(fp[len(fp)-4 : len(fp)]) {
		case ".bmp":
			sr.Files[i].MimeType = "image/bmp"
		case ".gif":
			sr.Files[i].MimeType = "image/gif"
		case ".pdf":
			sr.Files[i].MimeType = "application/pdf"
		case ".png":
			sr.Files[i].MimeType = "image/png"
		case ".jpg":
			sr.Files[i].MimeType = "image/jpg"
		default:
			if len(fp) >= 5 && strings.ToLower(fp[len(fp)-5:len(fp)]) == ".jpeg" {
				sr.Files[i].MimeType = "image/jpeg"
			} else {
				return nil, fmt.Errorf("failed to infer MIME type from file path: %v", fp)
			}
		}
	}
	for i, fp := range filePaths {
		fileContents, err := ioutil.ReadFile(fp)
		if err != nil {
			return nil, err
		}
		sr.Files[i].Base64File = base64.StdEncoding.EncodeToString(fileContents)
	}
	buf, err := json.Marshal(&sr)
	if err != nil {
		return nil, err
	}
	// TODO: batch into 8-file requests
	req, err := http.NewRequest("POST", fmt.Sprintf("https://siftrics.com/api/hydra/%v/", dataSourceId), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", fmt.Sprintf("Basic %v", c.apiKey))
	var httpClient http.Client
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("Invalid API key; Received 401 Unauthorized from initial HTTP request to the Hydra API.\n")
	} else if resp.StatusCode == 404 {
		return nil, fmt.Errorf("Received 404 Not Found --- Invalid data source ID. (Note that the name of the data source is NOT necessarily the ID of the data source. The ID of the data source is listed on its page on siftrics.com. Spaces are usually replaced by hyphens.)\n")
	} else if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Non-200 response from intial HTTP request to the Hydra API. Status of inital HTTP response: %v. Furthermore, failed to read body of initial HTTP response.", resp.StatusCode)
		}
		return nil, fmt.Errorf("Non-200 response from intial HTTP request to the Hydra API. Status of inital HTTP response: %v. Body of initial HTTP response:\n%v", resp.StatusCode, string(body))
	}
	var rfs RecognizedFiles
	if err := json.NewDecoder(resp.Body).Decode(&rfs); err != nil {
		return nil, fmt.Errorf("This should never happen and is not your fault: failed to decode body of initial HTTP request; error: %v", err)
	}

	filesChan := make(chan RecognizedFile, 16)
	go func() {
		for _, rf := range rfs.Rows {
			filesChan <- rf
		}
		close(filesChan)
	}()
	return filesChan, nil
}
