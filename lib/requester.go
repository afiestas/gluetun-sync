/* SPDX-License-Identifier: MIT */
package lib

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"text/template"
)

const (
	UnInitialized = 0
	Success       = 1
	Error         = 2
)

var forwardHeaders = []string{"Authorization"}

type RequesterError struct {
	Errors []error
}

func (m *RequesterError) Error() string {
	var errMsgs []string
	for _, err := range m.Errors {
		errMsgs = append(errMsgs, err.Error())
	}
	return strings.Join(errMsgs, "; ")
}

type StatusUpdate struct {
	Service string
	Method  string
	Path    string
	Status  int
	Step    int
	Error   error
}

type Requester struct {
	httpClient *http.Client
}

func NewRequester() Requester {
	return Requester{httpClient: http.DefaultClient}
}

func NewRequesterWithClient(client *http.Client) Requester {
	return Requester{httpClient: client}
}

type templateData struct {
	Credentials
	Port uint16
}

func executeTemplate(templateStr string, templateData templateData) (*bytes.Buffer, error) {
	templ, err := template.New("template").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse tempalte in url %w", err)
	}

	var result bytes.Buffer
	err = templ.Execute(&result, templateData)
	if err != nil {
		return nil, fmt.Errorf("error executing the template %w", err)
	}

	return &result, nil
}

func withMethod(method string) string {
	if method == "" {
		return "GET"
	}
	return method
}

func withUrl(urlTempl string, templateData templateData) (string, error) {
	url, err := executeTemplate(urlTempl, templateData)
	if err != nil {
		err := fmt.Errorf("coudln't execute template for url %s error: %v", url, err)
		return "", err
	}

	return url.String(), nil
}

type bodyInfo struct {
	ContentType string
	Data        *bytes.Buffer
}

func withBody(request Request, templateData templateData) (bodyInfo, error) {
	bodyInfo := bodyInfo{ContentType: request.ContentType, Data: nil}

	if bodyInfo.ContentType == "" {
		return bodyInfo, nil
	}
	if request.Payload == "" {
		return bodyInfo, fmt.Errorf("content type was set but no payload found %s", request.ContentType)
	}

	var err error
	bodyInfo.Data, err = executeTemplate(request.Payload, templateData)
	if err != nil {
		return bodyInfo, fmt.Errorf("coudln't execute template %s", request.ContentType)
	}

	return bodyInfo, nil
}

func reportUpdate(updateChan chan StatusUpdate, update StatusUpdate) {
	if updateChan == nil {
		return
	}
	updateChan <- update
}

func (r *Requester) SendRequests(port uint16, requests map[string]RequestGroup, updateChan chan StatusUpdate) error {
	errs := RequesterError{}

	addErr := func(err error, update StatusUpdate) {
		update.Error = err
		update.Status = Error
		reportUpdate(updateChan, update)
		errs.Errors = append(errs.Errors, err)
	}

	for service, requestGroup := range requests {
		headers := map[string][]string{}
		jar, _ := cookiejar.New(nil)
		r.httpClient.Jar = jar
		templateData := templateData{requestGroup.Credentials, port}

		for k, request := range requestGroup.Requests {
			update := StatusUpdate{Service: service, Method: request.Method, Path: request.Url, Step: k + 1, Status: UnInitialized}
			url, err := withUrl(request.Url, templateData)
			if err != nil {
				err = fmt.Errorf("cound't build url with tempalte %w", err)
				addErr(err, update)
				break
			}

			bodyInfo, err := withBody(request, templateData)
			if err != nil {
				err = fmt.Errorf("content type was set but no payload found %s", request.ContentType)
				addErr(err, update)
				break
			}

			var body io.Reader
			if bodyInfo.Data != nil {
				body = bodyInfo.Data
			}

			req, _ := http.NewRequest(withMethod(request.Method), url, body)
			req.Header = http.Header(headers)
			if bodyInfo.ContentType != "" {
				req.Header.Set("Content-Type", bodyInfo.ContentType)
			}

			r, err := r.httpClient.Do(req)
			if err != nil {
				addErr(err, update)
				break
			}

			update.Status = Success
			reportUpdate(updateChan, update)
			for _, header := range forwardHeaders {
				if auth := r.Header.Get(header); auth != "" {
					headers[header] = []string{auth}
				}
			}
		}
	}

	if updateChan != nil {
		close(updateChan)
	}

	if len(errs.Errors) == 0 {
		return nil
	}

	return &errs
}
