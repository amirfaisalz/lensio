package authz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ErrPermissionDenied  = errors.New("permission denied")
	ErrInvalidResource   = errors.New("invalid resource specification")
	ErrInvalidSubject    = errors.New("invalid subject specification")
	ErrInvalidPermission = errors.New("invalid permission")
	ErrAuthorizerFailed  = errors.New("authorizer service communication failed")
)

var validSpiceDBIDRegex = regexp.MustCompile(`^[a-zA-Z0-9/_|\-=+]+$`)

// Resource identifies an entity being secured in the Zanzibar domain model.
type Resource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// NewResource creates a new Resource descriptor.
func NewResource(resourceType, id string) Resource {
	return Resource{Type: strings.TrimSpace(resourceType), ID: strings.TrimSpace(id)}
}

func (r Resource) String() string {
	return fmt.Sprintf("%s:%s", r.Type, r.ID)
}

// Subject identifies the actor or principal requesting access.
type Subject struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Relation string `json:"relation,omitempty"`
}

// NewSubject creates a new Subject descriptor.
func NewSubject(subjectType, id string) Subject {
	return Subject{Type: strings.TrimSpace(subjectType), ID: strings.TrimSpace(id)}
}

func (s Subject) String() string {
	if s.Relation != "" {
		return fmt.Sprintf("%s:%s#%s", s.Type, s.ID, s.Relation)
	}
	return fmt.Sprintf("%s:%s", s.Type, s.ID)
}

// Relationship represents a Zanzibar relationship tuple linking a resource to a subject.
type Relationship struct {
	Resource Resource `json:"resource"`
	Relation string   `json:"relation"`
	Subject  Subject  `json:"subject"`
}

// NewRelationship creates a new Relationship tuple.
func NewRelationship(resource Resource, relation string, subject Subject) Relationship {
	return Relationship{
		Resource: resource,
		Relation: strings.TrimSpace(relation),
		Subject:  subject,
	}
}

// Authorizer defines the contract for Zanzibar relationship-based access control.
type Authorizer interface {
	CheckPermission(ctx context.Context, resource Resource, permission string, subject Subject) (bool, error)
	WriteRelationship(ctx context.Context, relation Relationship) error
}

// SanitizeID normalizes an identifier (such as email, username, or subject ID)
// to satisfy SpiceDB's strict object ID regex: ^(([a-zA-Z0-9/_|\-=+]{1,})|\*)$
func SanitizeID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "unknown"
	}
	if validSpiceDBIDRegex.MatchString(raw) {
		return raw
	}
	// Replace invalid characters (@, ., spaces, colons) with underscores
	var buf strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '/' || r == '_' || r == '|' || r == '-' || r == '=' || r == '+' || r == '*' {
			buf.WriteRune(r)
		} else {
			buf.WriteRune('_')
		}
	}
	return buf.String()
}

// Client is a production-grade HTTP adapter for SpiceDB v1 REST API.
type Client struct {
	endpoint     string
	presharedKey string
	httpClient   *http.Client
}

// NewClient constructs a SpiceDB HTTP client adapter.
func NewClient(endpoint, presharedKey string, httpClient *http.Client) *Client {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{
		endpoint:     endpoint,
		presharedKey: strings.TrimSpace(presharedKey),
		httpClient:   httpClient,
	}
}

// CheckPermission evaluates whether the subject has the specified permission on the resource.
func (c *Client) CheckPermission(ctx context.Context, resource Resource, permission string, subject Subject) (bool, error) {
	if resource.Type == "" || resource.ID == "" {
		return false, ErrInvalidResource
	}
	if subject.Type == "" || subject.ID == "" {
		return false, ErrInvalidSubject
	}
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return false, ErrInvalidPermission
	}

	reqPayload := map[string]any{
		"consistency": map[string]any{
			"fullyConsistent": true,
		},
		"resource": map[string]any{
			"objectType": resource.Type,
			"objectId":   SanitizeID(resource.ID),
		},
		"permission": permission,
		"subject": map[string]any{
			"object": map[string]any{
				"objectType": subject.Type,
				"objectId":   SanitizeID(subject.ID),
			},
			"optionalRelation": subject.Relation,
		},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return false, fmt.Errorf("marshal check request: %w", err)
	}

	url := c.endpoint + "/v1/permissions/check"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return false, fmt.Errorf("create check request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.presharedKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.presharedKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrAuthorizerFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("%w: status %d: %s", ErrAuthorizerFailed, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var res struct {
		Permissionship string `json:"permissionship"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return false, fmt.Errorf("decode check response: %w", err)
	}

	return res.Permissionship == "PERMISSIONSHIP_HAS_PERMISSION", nil
}

// WriteRelationship writes or touches a relationship in SpiceDB.
func (c *Client) WriteRelationship(ctx context.Context, relation Relationship) error {
	if relation.Resource.Type == "" || relation.Resource.ID == "" {
		return ErrInvalidResource
	}
	if relation.Subject.Type == "" || relation.Subject.ID == "" {
		return ErrInvalidSubject
	}
	relation.Relation = strings.TrimSpace(relation.Relation)
	if relation.Relation == "" {
		return errors.New("relation name is required")
	}

	reqPayload := map[string]any{
		"updates": []map[string]any{
			{
				"operation": "OPERATION_TOUCH",
				"relationship": map[string]any{
					"resource": map[string]any{
						"objectType": relation.Resource.Type,
						"objectId":   SanitizeID(relation.Resource.ID),
					},
					"relation": relation.Relation,
					"subject": map[string]any{
						"object": map[string]any{
							"objectType": relation.Subject.Type,
							"objectId":   SanitizeID(relation.Subject.ID),
						},
						"optionalRelation": relation.Subject.Relation,
					},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return fmt.Errorf("marshal write relationship request: %w", err)
	}

	url := c.endpoint + "/v1/relationships/write"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create write relationship request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.presharedKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.presharedKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAuthorizerFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: status %d: %s", ErrAuthorizerFailed, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

// WriteSchema provisions or updates a Zanzibar schema on the SpiceDB instance.
func (c *Client) WriteSchema(ctx context.Context, schema string) error {
	reqPayload := map[string]any{
		"schema": schema,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return fmt.Errorf("marshal write schema request: %w", err)
	}

	url := c.endpoint + "/v1/schema/write"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create write schema request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.presharedKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.presharedKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAuthorizerFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: status %d: %s", ErrAuthorizerFailed, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

// MockAuthorizer provides an in-memory, zero-dependency implementation of Authorizer for tests.
type MockAuthorizer struct {
	mu            sync.RWMutex
	relationships []Relationship
	permissions   map[string]bool
	checkErr      error
	writeErr      error
	defaultAllow  bool
}

// NewMockAuthorizer initializes a thread-safe in-memory MockAuthorizer.
func NewMockAuthorizer() *MockAuthorizer {
	return &MockAuthorizer{
		permissions: make(map[string]bool),
	}
}

// SetDefaultAllow sets whether permissions not explicitly recorded are allowed by default.
func (m *MockAuthorizer) SetDefaultAllow(allow bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultAllow = allow
}

// SetCheckError configures an error to return on all CheckPermission calls.
func (m *MockAuthorizer) SetCheckError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkErr = err
}

// SetWriteError configures an error to return on all WriteRelationship calls.
func (m *MockAuthorizer) SetWriteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeErr = err
}

// Allow explicitly grants a permission for a given resource and subject.
func (m *MockAuthorizer) Allow(resource Resource, permission string, subject Subject) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s#%s@%s:%s", resource.Type, SanitizeID(resource.ID), permission, subject.Type, SanitizeID(subject.ID))
	m.permissions[key] = true
}

// Deny explicitly denies a permission for a given resource and subject.
func (m *MockAuthorizer) Deny(resource Resource, permission string, subject Subject) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s#%s@%s:%s", resource.Type, SanitizeID(resource.ID), permission, subject.Type, SanitizeID(subject.ID))
	m.permissions[key] = false
}

// WriteRelationship stores a relationship tuple in memory.
func (m *MockAuthorizer) WriteRelationship(ctx context.Context, relation Relationship) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	if relation.Resource.Type == "" || relation.Resource.ID == "" {
		return ErrInvalidResource
	}
	if relation.Subject.Type == "" || relation.Subject.ID == "" {
		return ErrInvalidSubject
	}
	m.relationships = append(m.relationships, relation)
	return nil
}

// CheckPermission evaluates permissions based on explicit overrides and recorded relationships.
func (m *MockAuthorizer) CheckPermission(ctx context.Context, resource Resource, permission string, subject Subject) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.checkErr != nil {
		return false, m.checkErr
	}

	sanitizedResID := SanitizeID(resource.ID)
	sanitizedSubID := SanitizeID(subject.ID)

	key := fmt.Sprintf("%s:%s#%s@%s:%s", resource.Type, sanitizedResID, permission, subject.Type, sanitizedSubID)
	if allowed, found := m.permissions[key]; found {
		return allowed, nil
	}

	// Dynamic Zanzibar relationship evaluation:
	// 1. project:manage_api_keys:
	//    - Admin of project
	//    - Admin of organization that owns the project
	if resource.Type == "project" && permission == "manage_api_keys" {
		for _, rel := range m.relationships {
			// Direct project admin
			if rel.Resource.Type == "project" && SanitizeID(rel.Resource.ID) == sanitizedResID &&
				rel.Relation == "admin" &&
				rel.Subject.Type == subject.Type && SanitizeID(rel.Subject.ID) == sanitizedSubID {
				return true, nil
			}
			// Project member is an organization; organization admin inherits manage_api_keys
			if rel.Resource.Type == "project" && SanitizeID(rel.Resource.ID) == sanitizedResID &&
				rel.Relation == "organization" && rel.Subject.Type == "organization" {
				orgID := SanitizeID(rel.Subject.ID)
				for _, oRel := range m.relationships {
					if oRel.Resource.Type == "organization" && SanitizeID(oRel.Resource.ID) == orgID &&
						oRel.Relation == "admin" &&
						oRel.Subject.Type == subject.Type && SanitizeID(oRel.Subject.ID) == sanitizedSubID {
						return true, nil
					}
				}
			}
		}
	}

	// 2. api_key:revoke:
	//    - Key creator
	//    - Admin of the project that owns the API key
	if resource.Type == "api_key" && permission == "revoke" {
		var projectID string
		for _, rel := range m.relationships {
			if rel.Resource.Type == "api_key" && SanitizeID(rel.Resource.ID) == sanitizedResID {
				if rel.Relation == "creator" &&
					rel.Subject.Type == subject.Type && SanitizeID(rel.Subject.ID) == sanitizedSubID {
					return true, nil
				}
				if rel.Relation == "project" {
					projectID = SanitizeID(rel.Subject.ID)
				}
			}
		}

		if projectID != "" {
			// Check if subject can manage_api_keys on the project
			projKey := fmt.Sprintf("project:%s#manage_api_keys@%s:%s", projectID, subject.Type, sanitizedSubID)
			if allowed, found := m.permissions[projKey]; found && allowed {
				return true, nil
			}
			for _, rel := range m.relationships {
				if rel.Resource.Type == "project" && SanitizeID(rel.Resource.ID) == projectID &&
					rel.Relation == "admin" &&
					rel.Subject.Type == subject.Type && SanitizeID(rel.Subject.ID) == sanitizedSubID {
					return true, nil
				}
			}
		}
	}

	// 3. api_key:use:
	//    - Anyone who has view permission on the parent project
	if resource.Type == "api_key" && permission == "use" {
		for _, rel := range m.relationships {
			if rel.Resource.Type == "api_key" && SanitizeID(rel.Resource.ID) == sanitizedResID && rel.Relation == "project" {
				projID := SanitizeID(rel.Subject.ID)
				for _, pRel := range m.relationships {
					if pRel.Resource.Type == "project" && SanitizeID(pRel.Resource.ID) == projID &&
						(pRel.Relation == "member" || pRel.Relation == "admin") &&
						pRel.Subject.Type == subject.Type && SanitizeID(pRel.Subject.ID) == sanitizedSubID {
						return true, nil
					}
				}
			}
		}
	}

	return m.defaultAllow, nil
}

// GetWrittenRelationships returns a copy of all recorded relationships.
func (m *MockAuthorizer) GetWrittenRelationships() []Relationship {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]Relationship, len(m.relationships))
	copy(res, m.relationships)
	return res
}

// Reset clears all in-memory relationships and permission rules.
func (m *MockAuthorizer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.relationships = nil
	m.permissions = make(map[string]bool)
	m.checkErr = nil
	m.writeErr = nil
	m.defaultAllow = false
}
