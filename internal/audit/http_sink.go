package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const httpSinkTimeout = 5 * time.Second

type HTTPSink struct {
	url    string
	client *http.Client
}

func NewHTTPSink(rawURL string) (*HTTPSink, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("некорректный URL приёмника аудита %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("URL приёмника аудита %q: ожидалась схема http или https", rawURL)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("URL приёмника аудита %q: отсутствует хост", rawURL)
	}

	return &HTTPSink{
		url: rawURL,
		client: &http.Client{
			Timeout:       httpSinkTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func (s *HTTPSink) Notify(ctx context.Context, e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("приёмник аудита ответил кодом %d", resp.StatusCode)
	}

	return nil
}
