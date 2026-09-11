package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
	"github.com/amirfaisalz/nusaid/apps/api/internal/quota"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

// UpdatePlanRequest represents the JSON payload to change an organization's subscription plan.
type UpdatePlanRequest struct {
	PlanCode string `json:"plan_code"`
}

// AccountDetailsHandler handles GET /api/v1/account.
func AccountDetailsHandler(accountStore store.AccountStore, defaultOrgID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := resolveOrgID(r, r.URL.Query().Get("org_id"), defaultOrgID)

		if accountStore == nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Account store unavailable")
			return
		}

		org, err := accountStore.GetOrganization(r.Context(), orgID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.ErrorWithRequest(w, r, http.StatusNotFound, response.CodeInvalidRequest, "Organization not found")
				return
			}
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to retrieve organization")
			return
		}

		response.JSON(w, http.StatusOK, map[string]any{
			"organization_id":   org.ID,
			"organization_name": org.Name,
			"slug":              org.Slug,
			"plan_code":         org.PlanCode,
			"plan_name":         org.PlanName,
			"active_keys_count": org.ActiveKeysCount,
			"created_at":        org.CreatedAt,
		})
	}
}

// AccountPlanHandler handles GET /api/v1/account/plan.
func AccountPlanHandler(accountStore store.AccountStore, defaultOrgID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := resolveOrgID(r, r.URL.Query().Get("org_id"), defaultOrgID)

		if accountStore == nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Account store unavailable")
			return
		}

		plan, err := accountStore.GetOrganizationPlan(r.Context(), orgID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.ErrorWithRequest(w, r, http.StatusNotFound, response.CodeInvalidRequest, "Plan not found")
				return
			}
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to retrieve organization plan")
			return
		}

		_, resetTime := quota.CurrentBillingCycle(time.Now())

		response.JSON(w, http.StatusOK, map[string]any{
			"plan_code":             plan.Code,
			"plan_name":             plan.Name,
			"monthly_quota":         plan.MonthlyQuota,
			"rate_limit_per_minute": plan.RateLimitPerMinute,
			"billing_cycle_reset":   resetTime,
		})
	}
}

// UpdatePlanHandler handles PUT /api/v1/account/plan.
func UpdatePlanHandler(accountStore store.AccountStore, auditStore store.AuditStore, defaultOrgID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := resolveOrgID(r, r.URL.Query().Get("org_id"), defaultOrgID)

		var req UpdatePlanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid JSON payload")
			return
		}

		req.PlanCode = strings.ToLower(strings.TrimSpace(req.PlanCode))
		if req.PlanCode == "" {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Field 'plan_code' is required")
			return
		}

		if accountStore == nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Account store unavailable")
			return
		}

		// Retrieve old plan for audit log metadata
		var oldPlanCode string
		if oldPlan, err := accountStore.GetOrganizationPlan(r.Context(), orgID); err == nil && oldPlan != nil {
			oldPlanCode = oldPlan.Code
		}

		if err := accountStore.UpdateOrganizationPlan(r.Context(), orgID, req.PlanCode); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid plan code or organization not found")
				return
			}
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to update organization plan")
			return
		}

		// Record audit log
		if auditStore != nil {
			actorID := "system"
			if key := middleware.GetAPIKey(r.Context()); key != nil && key.ID != "" {
				actorID = "api_key:" + key.ID
			}

			_ = auditStore.RecordAuditLog(r.Context(), &store.AuditLog{
				OrgID:          orgID,
				ActorID:        actorID,
				Action:         "plan.change",
				TargetResource: "organization:" + orgID,
				Metadata: map[string]any{
					"old_plan": oldPlanCode,
					"new_plan": req.PlanCode,
				},
			})
		}

		response.JSON(w, http.StatusOK, map[string]any{
			"message":   "Subscription plan updated successfully",
			"plan_code": req.PlanCode,
		})
	}
}
