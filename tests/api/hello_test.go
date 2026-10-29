//go:build integration

// Package api 端到端 HTTP 回归测试：验证「入口 -> APISIX -> BFF -> 下游服务」的关键链路。
//
// 运行（对部署好的环境，经网关统一入口）：
//
//	TEST_BASE_URL=https://<域名> go test -tags=integration ./tests/api -count=1 -v
package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type apiError struct {
	Code    int    `json:"code"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func baseURL() string {
	return strings.TrimRight(getenv("TEST_BASE_URL", "http://127.0.0.1:9080"), "/")
}

func postJSON(t *testing.T, path string, body any) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL()+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, out
}

func reason(body []byte) string {
	var e apiError
	_ = json.Unmarshal(body, &e)
	return e.Reason
}

// C 端手写 BFF：网关 → api-service（service → biz）→ hello-service
func TestConsumerSayHello(t *testing.T) {
	code, body := postJSON(t, "/api/v1/consumer/Hello/SayHello", map[string]string{"name": "api-test"})
	var reply struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &reply); code != http.StatusOK || err != nil || reply.Message != "Hello, api-test!" {
		t.Fatalf("say hello: %d %s", code, body)
	}
	// 参数校验在 proto 层（buf validate）：空 name 返回 400
	if code, body = postJSON(t, "/api/v1/consumer/Hello/SayHello", map[string]string{"name": ""}); code != http.StatusBadRequest || reason(body) == "" {
		t.Fatalf("validate: %d %s", code, body)
	}
}

// B 端纯代理：网关 → admin-service（proxyx）→ hello-service；响应是 snake_case 的 protojson
func TestAdminSayHelloAndList(t *testing.T) {
	if code, body := postJSON(t, "/api/v1/admin/Hello/SayHello", map[string]string{"name": "admin-test"}); code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	code, body := postJSON(t, "/api/v1/admin/Hello/ListGreeting", map[string]any{
		"name": "admin-test",
		"page": map[string]int{"current": 1, "size": 10},
	})
	var reply struct {
		Greetings []struct {
			Name      string `json:"name"`
			CreatedAt string `json:"created_at"`
		} `json:"greetings"`
		Total string `json:"total"` // int64 → protojson 字符串
	}
	if err := json.Unmarshal(body, &reply); code != http.StatusOK || err != nil || len(reply.Greetings) == 0 || reply.Greetings[0].Name != "admin-test" || reply.Greetings[0].CreatedAt == "" || reply.Total == "" {
		t.Fatalf("unexpected list: %d %s", code, body)
	}
}
