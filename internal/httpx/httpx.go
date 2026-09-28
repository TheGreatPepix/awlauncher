package httpx

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const BrowserAgent = "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.5414.120 Downloader/19160 MyComGameCenter/1916 Safari/537.36"

type StatusError struct {
	Host string
	Code int
}

func (e *StatusError) Error() string { return fmt.Sprintf("%s: HTTP %d", e.Host, e.Code) }

func (e *StatusError) ClientError() bool { return e.Code >= 400 && e.Code < 500 }

func Get(client *http.Client, url string, max int64) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("response too large: %s", url)
	}
	return data, nil
}

func Post(client *http.Client, endpoint, contentType string, body []byte, agent string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", agent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{Host: req.URL.Host, Code: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, errors.New("auth response too large")
	}
	return data, nil
}
