package authz_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/authz"
)

func TestDomainTypes(t *testing.T) {
	t.Run("Resource", func(t *testing.T) {
		res := authz.NewResource("project", "proj-123")
		if res.Type != "project" || res.ID != "proj-123" {
			t.Errorf("unexpected resource: %+v", res)
		}
		if res.String() != "project:proj-123" {
			t.Errorf("unexpected string: %s", res.String())
		}
	})

	t.Run("Subject", func(t *testing.T) {
		sub1 := authz.NewSubject("user", "usr-123")
		if sub1.Type != "user" || sub1.ID != "usr-123" {
			t.Errorf("unexpected subject: %+v", sub1)
		}
		if sub1.String() != "user:usr-123" {
			t.Errorf("unexpected string: %s", sub1.String())
		}

		sub2 := authz.Subject{Type: "group", ID: "engineers", Relation: "member"}
		if sub2.String() != "group:engineers#member" {
			t.Errorf("unexpected string: %s", sub2.String())
		}
	})

	t.Run("Relationship", func(t *testing.T) {
		res := authz.NewResource("project", "proj-1")
		sub := authz.NewSubject("user", "usr-1")
		rel := authz.NewRelationship(res, "admin", sub)

		if rel.Resource != res || rel.Relation != "admin" || rel.Subject != sub {
			t.Errorf("unexpected relationship: %+v", rel)
		}
	})
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "unknown"},
		{"   ", "unknown"},
		{"admin_123", "admin_123"},
		{"admin-123", "admin-123"},
		{"00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000001"},
		{"admin@lensio.dev", "admin_lensio_dev"},
		{"developer.name+tag@example.com", "developer_name+tag_example_com"},
		{"user with spaces", "user_with_spaces"},
		{"invalid:::chars???", "invalid___chars___"},
		{"*", "*"},
	}

	for _, tc := range tests {
		got := authz.SanitizeID(tc.input)
		if got != tc.expected {
			t.Errorf("SanitizeID(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestClient_CheckPermission(t *testing.T) {
	t.Run("has permission", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/permissions/check" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("unexpected auth: %s", r.Header.Get("Authorization"))
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"permissionship": "PERMISSIONSHIP_HAS_PERMISSION",
			})
		}))
		defer ts.Close()

		client := authz.NewClient(ts.URL, "test-key", nil)
		allowed, err := client.CheckPermission(context.Background(), authz.NewResource("project", "p1"), "manage_api_keys", authz.NewSubject("user", "u1"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Fatal("expected permission to be allowed")
		}
	})

	t.Run("no permission", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"permissionship": "PERMISSIONSHIP_NO_PERMISSION",
			})
		}))
		defer ts.Close()

		client := authz.NewClient(ts.URL, "test-key", nil)
		allowed, err := client.CheckPermission(context.Background(), authz.NewResource("project", "p1"), "manage_api_keys", authz.NewSubject("user", "u2"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if allowed {
			t.Fatal("expected permission to be denied")
		}
	})

	t.Run("validation errors", func(t *testing.T) {
		client := authz.NewClient("http://dummy", "key", nil)
		ctx := context.Background()

		if _, err := client.CheckPermission(ctx, authz.Resource{}, "perm", authz.NewSubject("user", "u")); !errors.Is(err, authz.ErrInvalidResource) {
			t.Errorf("expected ErrInvalidResource, got %v", err)
		}
		if _, err := client.CheckPermission(ctx, authz.NewResource("res", "1"), "perm", authz.Subject{}); !errors.Is(err, authz.ErrInvalidSubject) {
			t.Errorf("expected ErrInvalidSubject, got %v", err)
		}
		if _, err := client.CheckPermission(ctx, authz.NewResource("res", "1"), "", authz.NewSubject("user", "u")); !errors.Is(err, authz.ErrInvalidPermission) {
			t.Errorf("expected ErrInvalidPermission, got %v", err)
		}
	})

	t.Run("server error and malformed responses", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("spicedb internal error"))
		}))
		defer ts.Close()

		client := authz.NewClient(ts.URL, "key", nil)
		_, err := client.CheckPermission(context.Background(), authz.NewResource("p", "1"), "view", authz.NewSubject("u", "1"))
		if err == nil || !errors.Is(err, authz.ErrAuthorizerFailed) {
			t.Fatalf("expected ErrAuthorizerFailed, got %v", err)
		}

		// Malformed JSON
		tsMalformed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{invalid-json"))
		}))
		defer tsMalformed.Close()

		clientMalformed := authz.NewClient(tsMalformed.URL, "key", nil)
		_, err = clientMalformed.CheckPermission(context.Background(), authz.NewResource("p", "1"), "view", authz.NewSubject("u", "1"))
		if err == nil {
			t.Fatal("expected error on malformed json")
		}
	})

	t.Run("network connection failure", func(t *testing.T) {
		client := authz.NewClient("http://127.0.0.1:59999", "key", nil)
		_, err := client.CheckPermission(context.Background(), authz.NewResource("p", "1"), "view", authz.NewSubject("u", "1"))
		if err == nil || !errors.Is(err, authz.ErrAuthorizerFailed) {
			t.Fatalf("expected network failure ErrAuthorizerFailed, got %v", err)
		}
	})
}

func TestClient_WriteRelationship(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/relationships/write" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"writtenAt": map[string]any{"token": "token123"},
			})
		}))
		defer ts.Close()

		client := authz.NewClient(ts.URL, "key", nil)
		rel := authz.NewRelationship(authz.NewResource("project", "p1"), "admin", authz.NewSubject("user", "u1"))
		if err := client.WriteRelationship(context.Background(), rel); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("validation errors", func(t *testing.T) {
		client := authz.NewClient("http://dummy", "key", nil)
		ctx := context.Background()

		if err := client.WriteRelationship(ctx, authz.Relationship{}); !errors.Is(err, authz.ErrInvalidResource) {
			t.Errorf("expected ErrInvalidResource, got %v", err)
		}
		if err := client.WriteRelationship(ctx, authz.Relationship{Resource: authz.NewResource("p", "1")}); !errors.Is(err, authz.ErrInvalidSubject) {
			t.Errorf("expected ErrInvalidSubject, got %v", err)
		}
		if err := client.WriteRelationship(ctx, authz.Relationship{Resource: authz.NewResource("p", "1"), Subject: authz.NewSubject("u", "1")}); err == nil {
			t.Error("expected error for empty relation")
		}
	})

	t.Run("server error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid relationship"))
		}))
		defer ts.Close()

		client := authz.NewClient(ts.URL, "key", nil)
		rel := authz.NewRelationship(authz.NewResource("project", "p1"), "admin", authz.NewSubject("user", "u1"))
		err := client.WriteRelationship(context.Background(), rel)
		if err == nil || !errors.Is(err, authz.ErrAuthorizerFailed) {
			t.Fatalf("expected ErrAuthorizerFailed, got %v", err)
		}
	})

	t.Run("network failure", func(t *testing.T) {
		client := authz.NewClient("http://127.0.0.1:59999", "key", nil)
		rel := authz.NewRelationship(authz.NewResource("project", "p1"), "admin", authz.NewSubject("user", "u1"))
		err := client.WriteRelationship(context.Background(), rel)
		if err == nil || !errors.Is(err, authz.ErrAuthorizerFailed) {
			t.Fatalf("expected ErrAuthorizerFailed, got %v", err)
		}
	})
}

func TestClient_WriteSchema(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/schema/write" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"writtenAt": map[string]any{"token": "token123"},
			})
		}))
		defer ts.Close()

		client := authz.NewClient(ts.URL, "key", nil)
		if err := client.WriteSchema(context.Background(), "definition user {}"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("syntax error in schema"))
		}))
		defer ts.Close()

		client := authz.NewClient(ts.URL, "key", nil)
		err := client.WriteSchema(context.Background(), "bad schema")
		if err == nil || !errors.Is(err, authz.ErrAuthorizerFailed) {
			t.Fatalf("expected ErrAuthorizerFailed, got %v", err)
		}
	})

	t.Run("network error", func(t *testing.T) {
		client := authz.NewClient("http://127.0.0.1:59999", "key", nil)
		err := client.WriteSchema(context.Background(), "definition user {}")
		if err == nil || !errors.Is(err, authz.ErrAuthorizerFailed) {
			t.Fatalf("expected ErrAuthorizerFailed, got %v", err)
		}
	})
}

func TestMockAuthorizer(t *testing.T) {
	ctx := context.Background()
	mock := authz.NewMockAuthorizer()

	userAdmin := authz.NewSubject("user", "admin@lensio.dev")
	userDev := authz.NewSubject("user", "dev@veriform.com")
	projectRes := authz.NewResource("project", "proj-main")
	orgRes := authz.NewResource("organization", "org-lensio")
	keyRes := authz.NewResource("api_key", "key-live-123")

	// 1. Default allow/deny
	mock.SetDefaultAllow(false)
	allowed, err := mock.CheckPermission(ctx, projectRes, "manage_api_keys", userAdmin)
	if err != nil || allowed {
		t.Fatalf("expected false with defaultAllow=false, got %v (err: %v)", allowed, err)
	}

	mock.SetDefaultAllow(true)
	allowed, err = mock.CheckPermission(ctx, projectRes, "manage_api_keys", userAdmin)
	if err != nil || !allowed {
		t.Fatalf("expected true with defaultAllow=true, got %v (err: %v)", allowed, err)
	}
	mock.SetDefaultAllow(false)

	// 2. Explicit Allow / Deny overrides
	mock.Allow(projectRes, "manage_api_keys", userAdmin)
	allowed, _ = mock.CheckPermission(ctx, projectRes, "manage_api_keys", userAdmin)
	if !allowed {
		t.Fatal("expected explicit allow to return true")
	}

	mock.Deny(projectRes, "manage_api_keys", userAdmin)
	allowed, _ = mock.CheckPermission(ctx, projectRes, "manage_api_keys", userAdmin)
	if allowed {
		t.Fatal("expected explicit deny to return false")
	}

	// 3. Reset
	mock.Reset()

	// 4. Zanzibar relationship: Direct project admin has manage_api_keys
	_ = mock.WriteRelationship(ctx, authz.NewRelationship(projectRes, "admin", userAdmin))
	allowed, err = mock.CheckPermission(ctx, projectRes, "manage_api_keys", userAdmin)
	if err != nil || !allowed {
		t.Fatalf("expected direct project admin to have manage_api_keys: %v", err)
	}

	// Dev is not an admin
	allowed, _ = mock.CheckPermission(ctx, projectRes, "manage_api_keys", userDev)
	if allowed {
		t.Fatal("expected dev without relation to not have manage_api_keys")
	}

	// 5. Zanzibar relationship: Inherited from organization admin
	mock.Reset()
	orgSubject := authz.Subject{Type: "organization", ID: orgRes.ID}
	_ = mock.WriteRelationship(ctx, authz.NewRelationship(projectRes, "organization", orgSubject))
	_ = mock.WriteRelationship(ctx, authz.NewRelationship(orgRes, "admin", userAdmin))

	allowed, err = mock.CheckPermission(ctx, projectRes, "manage_api_keys", userAdmin)
	if err != nil || !allowed {
		t.Fatalf("expected org admin to inherit manage_api_keys on project: %v", err)
	}

	// 6. Zanzibar relationship: api_key creator can revoke
	mock.Reset()
	_ = mock.WriteRelationship(ctx, authz.NewRelationship(keyRes, "creator", userDev))
	allowed, err = mock.CheckPermission(ctx, keyRes, "revoke", userDev)
	if err != nil || !allowed {
		t.Fatalf("expected key creator to have revoke permission: %v", err)
	}

	// Admin of the project that owns the key can also revoke
	projSubject := authz.Subject{Type: "project", ID: projectRes.ID}
	_ = mock.WriteRelationship(ctx, authz.NewRelationship(keyRes, "project", projSubject))
	_ = mock.WriteRelationship(ctx, authz.NewRelationship(projectRes, "admin", userAdmin))

	allowed, err = mock.CheckPermission(ctx, keyRes, "revoke", userAdmin)
	if err != nil || !allowed {
		t.Fatalf("expected project admin to have revoke permission on api_key: %v", err)
	}

	// Test project-level manage_api_keys explicit permission grant triggers key revoke
	userDevLead := authz.NewSubject("user", "lead@lensio.dev")
	mock.Allow(projectRes, "manage_api_keys", userDevLead)
	allowed, err = mock.CheckPermission(ctx, keyRes, "revoke", userDevLead)
	if err != nil || !allowed {
		t.Fatalf("expected user with project manage_api_keys override to revoke key: %v", err)
	}

	// Stranger cannot revoke
	stranger := authz.NewSubject("user", "stranger")
	allowed, _ = mock.CheckPermission(ctx, keyRes, "revoke", stranger)
	if allowed {
		t.Fatal("expected stranger to not have revoke permission")
	}

	// 7. api_key:use: project member can use
	_ = mock.WriteRelationship(ctx, authz.NewRelationship(projectRes, "member", userDev))
	allowed, err = mock.CheckPermission(ctx, keyRes, "use", userDev)
	if err != nil || !allowed {
		t.Fatalf("expected project member to have use permission on api_key: %v", err)
	}

	// 8. GetWrittenRelationships
	written := mock.GetWrittenRelationships()
	if len(written) != 4 {
		t.Fatalf("expected 4 recorded relationships, got %d", len(written))
	}

	// 9. Write validation errors
	if err := mock.WriteRelationship(ctx, authz.Relationship{}); !errors.Is(err, authz.ErrInvalidResource) {
		t.Errorf("expected ErrInvalidResource, got %v", err)
	}
	if err := mock.WriteRelationship(ctx, authz.Relationship{Resource: authz.NewResource("p", "1")}); !errors.Is(err, authz.ErrInvalidSubject) {
		t.Errorf("expected ErrInvalidSubject, got %v", err)
	}

	// 10. Configured errors
	simulatedErr := errors.New("simulated failure")
	mock.SetCheckError(simulatedErr)
	if _, err := mock.CheckPermission(ctx, projectRes, "manage_api_keys", userAdmin); !errors.Is(err, simulatedErr) {
		t.Fatalf("expected simulated error, got %v", err)
	}

	mock.SetWriteError(simulatedErr)
	if err := mock.WriteRelationship(ctx, authz.NewRelationship(projectRes, "admin", userAdmin)); !errors.Is(err, simulatedErr) {
		t.Fatalf("expected simulated write error, got %v", err)
	}
}

func BenchmarkSanitizeID(b *testing.B) {
	id := "admin.developer+finance@lensio.dev"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = authz.SanitizeID(id)
	}
}

func BenchmarkMockAuthorizer_CheckPermission(b *testing.B) {
	mock := authz.NewMockAuthorizer()
	user := authz.NewSubject("user", "admin@lensio.dev")
	proj := authz.NewResource("project", "00000000-0000-0000-0000-000000000001")
	_ = mock.WriteRelationship(context.Background(), authz.NewRelationship(proj, "admin", user))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mock.CheckPermission(ctx, proj, "manage_api_keys", user)
	}
}

func BenchmarkMockAuthorizer_WriteRelationship(b *testing.B) {
	mock := authz.NewMockAuthorizer()
	user := authz.NewSubject("user", "u1")
	proj := authz.NewResource("project", "p1")
	rel := authz.NewRelationship(proj, "admin", user)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mock.WriteRelationship(ctx, rel)
	}
}
