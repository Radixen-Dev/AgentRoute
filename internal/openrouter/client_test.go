// SPDX-License-Identifier: GPL-3.0-only

package openrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchModelsSortsByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing/incorrect Authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"zeta/model","name":"Zeta","context_length":8000,"pricing":{"prompt":"0.000001","completion":"0.000002"}},
			{"id":"alpha/model","name":"Alpha","context_length":128000,"pricing":{"prompt":"0.000003","completion":"0.000004"}}
		]}`))
	}))
	defer srv.Close()

	c := NewClient("test-key")
	c.HTTPClient = srv.Client()
	c.BaseURL = srv.URL

	models, err := c.FetchModels(context.Background())
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].ID != "alpha/model" || models[1].ID != "zeta/model" {
		t.Fatalf("models not sorted by ID: %+v", models)
	}
}

func TestFetchModelsNoAPIKey(t *testing.T) {
	c := NewClient("")
	if _, err := c.FetchModels(context.Background()); err != ErrNoAPIKey {
		t.Fatalf("expected ErrNoAPIKey, got %v", err)
	}
}
