package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
)

func TestRequestLogging(t *testing.T) {
	var buf bytes.Buffer
	h := NewRouter(Services{}, slog.New(slog.NewJSONHandler(&buf, nil)))

	send(h, http.MethodGet, "/health", "") // debug level: not logged at info
	send(h, http.MethodGet, "/nope?x=1", "")

	var line struct {
		Msg    string `json:"msg"`
		Method string `json:"method"`
		Path   string `json:"path"`
		Status int    `json:"status"`
	}
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1:\n%s", len(lines), buf.String())
	}
	if err := json.Unmarshal(lines[0], &line); err != nil {
		t.Fatal(err)
	}
	if line.Msg != "request" || line.Method != "GET" || line.Path != "/nope?x=1" || line.Status != http.StatusNotFound {
		t.Errorf("log line = %s", lines[0])
	}
}
