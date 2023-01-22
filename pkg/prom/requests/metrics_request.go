package requests

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

var (
	ErrResponseNotOK error = errors.New("response not ok")
)

type (
	Requests interface {
		Request(ctx context.Context, uPath string, params map[string]string) ([]byte, error)
	}

	v1 struct {
		baseUrl string
		client  *http.Client
		timeout time.Duration
	}
)

func CreateURL(baseUrl string, uPath string, params map[string]string) (string, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, uPath)
	url := u.String()

	count := 0
	for value, key := range params {
		if count == 0 {
			url += "?" + value + "=" + key
			count += 1
		} else {
			url += "&" + value + "=" + key
		}
	}
	return url, nil
}

func (v *v1) Request(ctx context.Context, uPath string, params map[string]string) ([]byte, error) {

	url, err := CreateURL(v.baseUrl, uPath, params)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w. %s", ErrResponseNotOK, http.StatusText(resp.StatusCode))
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return bodyBytes, nil
}

func New(url string, client *http.Client, timeout time.Duration) *v1 {
	return &v1{
		baseUrl: url,
		client:  client,
		timeout: timeout,
	}
}
