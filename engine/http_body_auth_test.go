package engine

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"requestflow/model"
)

// TestRequestBodyTypes 验证各 Body 类型的编码与 Content-Type。
func TestRequestBodyTypes(t *testing.T) {
	type got struct {
		method, contentType, body string
		query                     string
	}
	var received got
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received = got{
			method:      r.Method,
			contentType: r.Header.Get("Content-Type"),
			body:        string(b),
			query:       r.URL.RawQuery,
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := NewHTTPClient(5*time.Second, false)

	// raw JSON：自动加 application/json
	_, err := c.DoRequest(&model.Step{BodyType: "raw", Body: `{"a":1}`}, "POST", srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(received.contentType, "application/json") || received.body != `{"a":1}` {
		t.Fatalf("raw JSON: ct=%q body=%q", received.contentType, received.body)
	}

	// urlencoded
	_, err = c.DoRequest(&model.Step{BodyType: "urlencoded", FormData: []model.FormField{
		{Key: "user", Value: "张三"}, {Key: "page", Value: "2"},
	}}, "POST", srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(received.contentType, "application/x-www-form-urlencoded") ||
		!strings.Contains(received.body, "user=") || !strings.Contains(received.body, "page=2") {
		t.Fatalf("urlencoded: ct=%q body=%q", received.contentType, received.body)
	}

	// form-data
	_, err = c.DoRequest(&model.Step{BodyType: "form-data", FormData: []model.FormField{
		{Key: "name", Type: "text", Value: "abc"},
	}}, "POST", srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(received.contentType, "multipart/form-data") || !strings.Contains(received.body, "abc") {
		t.Fatalf("form-data: ct=%q body=%q", received.contentType, received.body)
	}

	// graphql
	_, err = c.DoRequest(&model.Step{BodyType: "graphql", GraphQLQuery: "query { me { id } }", GraphQLVariables: `{"x":1}`}, "POST", srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(received.contentType, "application/json") {
		t.Fatalf("graphql ct=%q", received.contentType)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(received.body), &payload); err != nil {
		t.Fatalf("graphql body 非 JSON: %v / %s", err, received.body)
	}
	if payload["query"] != "query { me { id } }" {
		t.Fatalf("graphql query 错误: %v", payload)
	}
}

// TestRequestAuth 验证 apikey（header/query）与 oauth2 认证。
func TestRequestAuth(t *testing.T) {
	var gotAuth, gotKeyHeader, gotKeyQuery string
	// token 端点
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"OAUTH-TOKEN","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotKeyHeader = r.Header.Get("X-API-Key")
		gotKeyQuery = r.URL.Query().Get("api_key")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := NewHTTPClient(5*time.Second, false)

	// apikey in header
	_, err := c.DoRequest(&model.Step{AuthType: "apikey", AuthKeyName: "X-API-Key", AuthKeyValue: "SECRET", AuthKeyIn: "header"}, "GET", srv.URL, nil, nil)
	if err != nil || gotKeyHeader != "SECRET" {
		t.Fatalf("apikey header: got=%q err=%v", gotKeyHeader, err)
	}

	// apikey in query
	_, err = c.DoRequest(&model.Step{AuthType: "apikey", AuthKeyName: "api_key", AuthKeyValue: "QSECRET", AuthKeyIn: "query"}, "GET", srv.URL, nil, nil)
	if err != nil || gotKeyQuery != "QSECRET" {
		t.Fatalf("apikey query: got=%q err=%v", gotKeyQuery, err)
	}

	// oauth2 client_credentials
	_, err = c.DoRequest(&model.Step{
		AuthType: "oauth2", AuthTokenURL: tokenSrv.URL,
		AuthClientID: "id", AuthClientSecret: "secret", AuthGrantType: "client_credentials",
	}, "GET", srv.URL, nil, nil)
	if err != nil || gotAuth != "Bearer OAUTH-TOKEN" {
		t.Fatalf("oauth2: auth=%q err=%v", gotAuth, err)
	}
}
