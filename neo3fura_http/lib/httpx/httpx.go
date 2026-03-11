package httpx

import (
	"io"
	"net/http"
	"time"
)

const DefaultTimeout = 15 * time.Second

var DefaultClient = &http.Client{
	Timeout: DefaultTimeout,
}

func Get(url string) (*http.Response, error) {
	return DefaultClient.Get(url)
}

func Post(url, contentType string, body io.Reader) (*http.Response, error) {
	return DefaultClient.Post(url, contentType, body)
}

func Do(req *http.Request) (*http.Response, error) {
	return DefaultClient.Do(req)
}
