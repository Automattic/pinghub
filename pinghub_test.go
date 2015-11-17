package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestApp(t *testing.T) {
	s := mockApp()
	defer s.Close()

	t.Log("Test: GET / serves HTML")
	resp := get(t, s, "/")
	body := responseBody(t, resp)
	_, err := regexp.Match("<html>", body)
	if err != nil {
		t.Error(err)
	}

	t.Log("Test: GET /somestring contains /somestring")
	resp = get(t, s, "/somestring")
	body = responseBody(t, resp)
	_, err = regexp.Match("/somestring", body)
	if err != nil {
		t.Error(err)
	}

	t.Log("Test: POST /testpath returns 200 OK")
	resp = post(t, s, "/testpath", "test message")
	if resp.Status != "200 OK" || string(responseBody(t, resp)) != "OK\n" {
		t.Errorf("POST response not 200 OK: %v", resp)
	}
}

func mockApp() *httptest.Server {
	return httptest.NewServer(newHandler())
}

func get(t *testing.T, s *httptest.Server, path string) *http.Response {
	resp, err := http.Get(s.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func responseBody(t *testing.T, r *http.Response) []byte {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func post(t *testing.T, s *httptest.Server, path string, message string) *http.Response {
	resp, err := http.Post(s.URL + path, "text/plain", strings.NewReader(message))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

