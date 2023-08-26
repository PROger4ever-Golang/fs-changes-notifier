package utils

import (
	"bytes"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

func NewHttpClient() *http.Client {
	tr := &http.Transport{
		// TLSClientConfig: &tls.Config{InsecureSkipVerify: true},

		MaxIdleConns:          1,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,

		DisableKeepAlives: false,
	}

	return &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}
}

func NewJsonRequest(url string, jsonBytes []byte) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, errors.Wrap(err, "http.NewRequest()")
	}

	req.Header.Add("Content-Type", "application/json")

	return req, nil
}

func SendRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	return resp, errors.Wrap(err, "client.Do(req)")
}
