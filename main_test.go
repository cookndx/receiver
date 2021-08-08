// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockBucket struct {
	WriterFn      func(string) io.WriteCloser
	WriterInvoked bool
}

type MockWriter struct{}

func (p *MockBucket) NewWriter(objKey string) io.WriteCloser {
	p.WriterInvoked = true
	if p.WriterFn != nil {
		return p.NewWriter(objKey)
	}

	var mockWriter io.WriteCloser
	mockWriter = &MockWriter{}
	return mockWriter
}

func (w *MockWriter) Write(p []byte) (n int, err error) {
	return 0, nil
}

func (w *MockWriter) Close() error {
	return nil
}

func (w *MockWriter) SetContentType(contentType string) {
	// no-op
}

func TestHelloHandler(t *testing.T) {
	tests := []struct {
		label string
		want  string
		name  string
	}{
		{
			label: "default",
			want:  "Hello World!\n",
			name:  "",
		},
		{
			label: "override",
			want:  "Hello Override!\n",
			name:  "Override",
		},
	}

	photos = &MockBucket{}

	originalName := os.Getenv("NAME")
	defer os.Setenv("NAME", originalName)

	for _, test := range tests {
		os.Setenv("NAME", test.name)

		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		helloHandler(rr, req)

		if got := rr.Body.String(); got != test.want {
			t.Errorf("%s: got %q, want %q", test.label, got, test.want)
		}
	}
}

func TestUploadGetHandler(t *testing.T) {
	tests := []struct {
		label      string
		wantPrefix string
		wantSuffix string
	}{
		{
			label:      "default",
			wantPrefix: "<!DOCTYPE html>\n<html>",
			wantSuffix: "</html>\n",
		},
	}

	photos = &MockBucket{}

	for _, test := range tests {
		req := httptest.NewRequest(http.MethodGet, "/upload", nil)
		rr := httptest.NewRecorder()
		uploadHandler(rr, req)

		if got := rr.Body.String(); strings.HasPrefix(got, test.wantPrefix) {
			t.Errorf("%s: got %q, want %q", test.label, got, test.wantPrefix)
		}
		if got := rr.Body.String(); strings.HasSuffix(got, test.wantSuffix) {
			t.Errorf("%s: got %q, want %q", test.label, got, test.wantSuffix)
		}
	}
}

func TestUploadPostHandler(t *testing.T) {
	fileName := "LICENSE"
	fileBytes, err := ioutil.ReadFile(fileName)
	assert.NoError(t, err)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("sourceFile", fileName)
	assert.NoError(t, err)
	_, err = part.Write(fileBytes)
	assert.NoError(t, err)

	err = writer.Close()
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	uploadHandler(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected %d, received %d", http.StatusCreated, rr.Code)
	}
}
