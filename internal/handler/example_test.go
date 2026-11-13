package handlers_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"practice/internal/audit"
	handlers "practice/internal/handler"
	"practice/internal/hash"
	models "practice/internal/model"
	"practice/internal/server"
	"practice/internal/storage"
)

type okPinger struct{}

func (okPinger) PingContext(context.Context) error { return nil }

func ExampleHandler_UpdateHandler() {
	srv := httptest.NewServer(server.NewRouter(storage.NewMemStorage(), audit.NewNotifier(), nil, ""))
	defer srv.Close()

	for _, path := range []string{
		"/update/gauge/Alloc/12.5",
		"/update/histogram/Alloc/12.5",
		"/update/counter/PollCount/abc",
	} {
		resp, err := http.Post(srv.URL+path, "text/plain", nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(strings.TrimSpace(resp.Status + " " + strings.TrimSpace(string(body))))
	}

	// Output:
	// 200 OK
	// 400 Bad Request Invalid metric type
	// 400 Bad Request Invalid metric value
}

func ExampleHandler_ValueHandler() {
	srv := httptest.NewServer(server.NewRouter(storage.NewMemStorage(), audit.NewNotifier(), nil, ""))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/update/gauge/Alloc/12.5", "text/plain", nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	resp.Body.Close()

	for _, path := range []string{"/value/gauge/Alloc", "/value/gauge/Unknown"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			fmt.Println(err)
			return
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(resp.Status, strings.TrimSpace(string(body)))
	}

	// Output:
	// 200 OK 12.5
	// 404 Not Found metric not found
}

func ExampleHandler_ListHandler() {
	srv := httptest.NewServer(server.NewRouter(storage.NewMemStorage(), audit.NewNotifier(), nil, ""))
	defer srv.Close()

	for _, path := range []string{"/update/gauge/Alloc/12.5", "/update/counter/PollCount/3"} {
		resp, err := http.Post(srv.URL+path, "text/plain", nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		resp.Body.Close()
	}

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(resp.Status)
	fmt.Println(resp.Header.Get("Content-Type"))
	fmt.Println(strings.TrimSpace(string(body)))

	// Output:
	// 200 OK
	// text/html; charset=utf-8
	// <!DOCTYPE html><html><head><meta charset="utf-8"><title>Metrics</title></head><body><h1>Metrics</h1><ul><li><b>Alloc</b> [Gauge] = 12.5</li><li><b>PollCount</b> [Counter] = 3</li></ul></body></html>
}

func ExampleHandler_UpdateJSONHandler() {
	srv := httptest.NewServer(server.NewRouter(storage.NewMemStorage(), audit.NewNotifier(), nil, ""))
	defer srv.Close()

	for _, payload := range []string{
		`{"id":"PollCount","type":"counter","delta":5}`,
		`{"id":"PollCount","type":"counter","delta":7}`,
		`{"id":"Alloc","type":"gauge"}`,
	} {
		resp, err := http.Post(srv.URL+"/update/", "application/json", strings.NewReader(payload))
		if err != nil {
			fmt.Println(err)
			return
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(resp.Status, strings.TrimSpace(string(body)))
	}

	// Output:
	// 200 OK {"id":"PollCount","type":"counter","delta":5}
	// 200 OK {"id":"PollCount","type":"counter","delta":12}
	// 400 Bad Request value is required for gauge
}

func ExampleHandler_ValueJSONHandler() {
	srv := httptest.NewServer(server.NewRouter(storage.NewMemStorage(), audit.NewNotifier(), nil, ""))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/update/", "application/json",
		strings.NewReader(`{"id":"Alloc","type":"gauge","value":12.5}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	resp.Body.Close()

	for _, payload := range []string{
		`{"id":"Alloc","type":"gauge"}`,
		`{"id":"Unknown","type":"gauge"}`,
	} {
		resp, err := http.Post(srv.URL+"/value/", "application/json", strings.NewReader(payload))
		if err != nil {
			fmt.Println(err)
			return
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(resp.Status, strings.TrimSpace(string(body)))
	}

	// Output:
	// 200 OK {"id":"Alloc","type":"gauge","value":12.5}
	// 404 Not Found metric not found
}

func ExampleHandler_UpdatesJSONHandler() {
	srv := httptest.NewServer(server.NewRouter(storage.NewMemStorage(), audit.NewNotifier(), nil, ""))
	defer srv.Close()

	for _, payload := range []string{
		`[{"id":"Alloc","type":"gauge","value":12.5},{"id":"PollCount","type":"counter","delta":3}]`,
		`[{"id":"HeapAlloc","type":"gauge","value":1},{"id":"Bad","type":"histogram","value":1}]`,
	} {
		resp, err := http.Post(srv.URL+"/updates/", "application/json", strings.NewReader(payload))
		if err != nil {
			fmt.Println(err)
			return
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(strings.TrimSpace(resp.Status + " " + strings.TrimSpace(string(body))))
	}

	for _, path := range []string{"/value/counter/PollCount", "/value/gauge/HeapAlloc"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			fmt.Println(err)
			return
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(resp.Status, strings.TrimSpace(string(body)))
	}

	// Output:
	// 200 OK
	// 400 Bad Request invalid metric type
	// 200 OK 3
	// 404 Not Found metric not found
}

func ExampleHandler_UpdatesJSONHandler_agent() {
	const key = "secret"

	srv := httptest.NewServer(server.NewRouter(storage.NewMemStorage(), audit.NewNotifier(), nil, key))
	defer srv.Close()

	batch := []models.Metrics{
		models.NewGaugeDTO("Alloc", 12.5),
		models.NewCounterDTO("PollCount", 1),
	}
	raw, err := json.Marshal(batch)
	if err != nil {
		fmt.Println(err)
		return
	}

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(raw); err != nil {
		fmt.Println(err)
		return
	}
	if err := zw.Close(); err != nil {
		fmt.Println(err)
		return
	}

	for _, sign := range []string{hash.Sum(key, raw), hash.Sum("wrong", raw)} {
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/updates/", bytes.NewReader(gz.Bytes()))
		if err != nil {
			fmt.Println(err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set(hash.Header, sign)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(strings.TrimSpace(resp.Status+" "+strings.TrimSpace(string(body))),
			hash.Equal(key, body, resp.Header.Get(hash.Header)))
	}

	// Output:
	// 200 OK true
	// 400 Bad Request invalid hash true
}

func ExamplePingHandler_Ping() {
	srv := httptest.NewServer(server.NewRouter(storage.NewMemStorage(), audit.NewNotifier(), nil, ""))
	defer srv.Close()

	var db handlers.Pinger = okPinger{}
	srvDB := httptest.NewServer(server.NewRouter(storage.NewMemStorage(), audit.NewNotifier(), db, ""))
	defer srvDB.Close()

	for _, base := range []string{srv.URL, srvDB.URL} {
		resp, err := http.Get(base + "/ping")
		if err != nil {
			fmt.Println(err)
			return
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(strings.TrimSpace(resp.Status + " " + strings.TrimSpace(string(body))))
	}

	// Output:
	// 500 Internal Server Error database is not configured
	// 200 OK
}
