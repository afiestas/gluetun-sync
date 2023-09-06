/* SPDX-License-Identifier: MIT */
package lib

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"testing"
)

type MockTransport func(req *http.Request) (*http.Response, error)

func (m MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func TestSimpleRequest(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("could not create a cookie jar %v", err)
	}

	port := uint16(1337)
	client := &http.Client{Jar: jar}
	requester := NewRequesterWithClient(client)

	t.Run("Baisc HTTP Request", func(t *testing.T) {
		totalRequests := 0
		client.Transport = MockTransport(func(req *http.Request) (*http.Response, error) {
			totalRequests += 1
			errors := []error{}
			if req.Method != "GET" {
				errors = append(errors, fmt.Errorf("the expecetd method is GET but %s received instead", req.Method))
			}
			if req.URL.Scheme != "https" {
				errors = append(errors, fmt.Errorf("the expecetd schema is https but %s received instead", req.Method))
			}
			if req.Host != "foo.com:2121" {
				errors = append(errors, fmt.Errorf("the expecetd host is foo.com but %s received instead", req.Host))
			}
			if req.URL.Path != "/somepath" {
				errors = append(errors, fmt.Errorf("the expecetd path is /somepath but %s received instead", req.URL.Path))
			}

			if len(errors) != 0 {
				t.Fatal("received malformed request", errors)
			}
			return &http.Response{StatusCode: 200}, nil
		})

		requester.SendRequests(port, map[string]RequestGroup{
			"test": {
				Requests: []Request{{Url: "https://foo.com:2121/somepath"}},
			}}, nil)

		if totalRequests != 1 {
			t.Fatal("expected 1 request but 0 received")
		}
	})

	t.Run("Payload content-type form", func(t *testing.T) {
		totalRequests := 0
		client.Transport = MockTransport(func(req *http.Request) (*http.Response, error) {
			totalRequests += 1
			errors := []error{}
			err := req.ParseForm()
			if err != nil {
				errors = append(errors, fmt.Errorf("couldn't parse multipart form %s", err))
			}

			if req.PostForm.Get("port") != fmt.Sprint(port) {
				errors = append(errors, fmt.Errorf("the expecetd port is %d but %s received instead", port, req.Form.Get("port")))
			}

			if len(errors) != 0 {
				t.Fatal("received malformed request", errors)
			}
			return &http.Response{StatusCode: 200}, nil
		})

		requester.SendRequests(port, map[string]RequestGroup{
			"test": {
				Requests: []Request{{
					Method:      "POST",
					Url:         "https://foo.com:2121/somepath",
					ContentType: "application/x-www-form-urlencoded",
					Payload:     "port={{.Port}}",
				}}},
		}, nil)

		if totalRequests != 1 {
			t.Fatal("expected 1 request but 0 received")
		}
	})

	t.Run("Payload content-type json", func(t *testing.T) {
		totalRequests := 0
		client.Transport = MockTransport(func(req *http.Request) (*http.Response, error) {
			totalRequests += 1
			errors := []error{}
			data := map[string]string{}
			err := json.NewDecoder(req.Body).Decode(&data)
			if err != nil {
				errors = append(errors, fmt.Errorf("error parsing the received json %v", err))
			}
			if data["port"] != fmt.Sprint(port) {
				errors = append(errors, fmt.Errorf("the expecetd port is %d but %s received instead", port, req.Form.Get("port")))
			}

			if len(errors) != 0 {
				t.Fatal("received malformed request", errors)
			}
			return &http.Response{StatusCode: 200}, nil
		})

		requester.SendRequests(port, map[string]RequestGroup{
			"test": {
				Requests: []Request{{
					Method:      "POST",
					Url:         "https://foo.com:2121/somepath",
					ContentType: "application/json",
					Payload:     "{\"port\": \"{{.Port}}\"}",
				}}},
		}, nil)

		if totalRequests != 1 {
			t.Fatal("expected 1 request but 0 received")
		}
	})

	t.Run("Credentials templating", func(t *testing.T) {
		totalRequests := 0
		client.Transport = MockTransport(func(req *http.Request) (*http.Response, error) {
			totalRequests += 1
			errors := []error{}
			if user := req.URL.Query()["user"][0]; user != "user1" {
				errors = append(errors, fmt.Errorf("the expecetd query param user is user1 but %s received instead", user))
			}
			if pass := req.URL.Query()["pass"][0]; pass != "pass1" {
				errors = append(errors, fmt.Errorf("the expecetd query param pass is user1 but %s received instead", pass))
			}

			if len(errors) != 0 {
				t.Fatal("received malformed request", errors)
			}
			return &http.Response{StatusCode: 200}, nil
		})

		errs := requester.SendRequests(port, map[string]RequestGroup{
			"test": {
				Credentials: Credentials{Username: "user1", Password: "pass1"},
				Requests:    []Request{{Url: "http://f.com/?user={{.Username}}&pass={{.Password}}"}}},
		}, nil)

		if errs != nil && len(errs.(*RequesterError).Errors) != 0 {
			t.Fatal("SendRequests failed with some errors", errs)
		}

		if totalRequests != 1 {
			t.Fatal("expected 1 request but 0 received")
		}
	})

	t.Run("Cookie forwarding", func(t *testing.T) {
		totalRequests := 0
		cookie := &http.Cookie{
			Name:  "some-name",
			Value: "some-value",
		}

		client.Transport = MockTransport(func(req *http.Request) (*http.Response, error) {
			totalRequests += 1
			errors := []error{}

			r := http.Response{StatusCode: 200, Header: http.Header{}}
			if totalRequests == 1 {
				r.Header.Add("Set-Cookie", cookie.String())
			} else if totalRequests == 2 {
				cookie, err := req.Cookie("some-name")
				if err != nil {
					err := fmt.Errorf("expected second request to have a cookie some-name but couldn't fetch it %w", err)
					errors = append(errors, err)
				} else if cookie.Value != "some-value" {
					err := fmt.Errorf("expecetd second request with cookie some-name with some-value but  %s received instead", cookie.Value)
					errors = append(errors, err)
				}
			}

			if len(errors) != 0 {
				t.Fatal("received malformed request", errors)
			}

			return &r, nil
		})

		errs := requester.SendRequests(port, map[string]RequestGroup{
			"test": {
				Requests: []Request{{Url: "http://f.com"}, {Url: "http://f.com/2"}}},
		}, nil)

		if errs != nil && len(errs.(*RequesterError).Errors) != 0 {
			t.Fatal("SendRequests failed with some errors", errs)
		}

		if totalRequests != 2 {
			t.Fatalf("expected 2 request but %d received", totalRequests)
		}
	})

	t.Run("Header forwarding", func(t *testing.T) {
		totalRequests := 0
		client.Transport = MockTransport(func(req *http.Request) (*http.Response, error) {
			totalRequests += 1
			errors := []error{}

			r := http.Response{StatusCode: 200, Header: http.Header{}}
			if totalRequests == 1 {
				r.Header.Add("Authorization", "some-token")
			} else if totalRequests == 2 {
				auth := req.Header.Get("Authorization")
				if auth != "some-token" {
					err := fmt.Errorf("expecetd second request with header Authorization with some-token but  %s received instead", auth)
					errors = append(errors, err)
				}
			}

			if len(errors) != 0 {
				t.Fatal("received malformed request", errors)
			}

			return &r, nil
		})

		errs := requester.SendRequests(port, map[string]RequestGroup{
			"test": {
				Requests: []Request{{Url: "http://f.com"}, {Url: "http://f.com/2"}}},
		}, nil)

		if errs != nil && len(errs.(*RequesterError).Errors) != 0 {
			t.Fatal("SendRequests failed with some errors", errs)
		}

		if totalRequests != 2 {
			t.Fatalf("expected 2 request but %d received", totalRequests)
		}
	})

	t.Run("Stop group processing on error", func(t *testing.T) {
		totalRequests := 0
		client.Transport = MockTransport(func(req *http.Request) (*http.Response, error) {
			totalRequests += 1

			r := http.Response{StatusCode: 200, Header: http.Header{}}

			return &r, nil
		})

		errs := requester.SendRequests(port, map[string]RequestGroup{
			"test": {
				Requests: []Request{{Url: "url.com?{{.NotExisting}}"}, {Url: "http://should-not-execute.com/2"}},
			},
			"test2": {
				Requests: []Request{{Url: "http://f.com"}, {Url: "http://f.com/2"}},
			},
		}, nil)

		if errs != nil && len(errs.(*RequesterError).Errors) != 1 {
			t.Fatal("SendRequests failed with some errors", errs)
		}

		if totalRequests != 2 {
			t.Fatalf("expected 3 request but %d received", totalRequests)
		}
	})
}
