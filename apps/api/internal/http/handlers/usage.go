package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
	"github.com/amirfaisalz/nusaid/apps/api/internal/quota"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

// UsageSummaryHandler handles GET /api/v1/usage.
func UsageSummaryHandler(usageStore store.UsageStore, accountStore store.AccountStore, defaultOrgID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := resolveOrgID(r, r.URL.Query().Get("org_id"), defaultOrgID)

		planQuota := 100
		cycleStart, cycleReset := quota.CurrentBillingCycle(time.Now())

		if accountStore != nil {
			plan, err := accountStore.GetOrganizationPlan(r.Context(), orgID)
			if err == nil && plan != nil && plan.MonthlyQuota > 0 {
				planQuota = plan.MonthlyQuota
			}
		}

		if usageStore == nil {
			response.JSON(w, http.StatusOK, store.UsageSummary{
				TotalRequests:     0,
				SuccessCount:      0,
				ErrorCount:        0,
				QuotaLimit:        planQuota,
				QuotaRemaining:    planQuota,
				BillingCycleReset: cycleReset,
			})
			return
		}

		summary, err := usageStore.GetUsageSummary(r.Context(), orgID, cycleStart, planQuota, cycleReset)
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to retrieve usage summary")
			return
		}

		response.JSON(w, http.StatusOK, summary)
	}
}

// DailyUsageHandler handles GET /api/v1/usage/daily.
func DailyUsageHandler(usageStore store.UsageStore, defaultOrgID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := resolveOrgID(r, r.URL.Query().Get("org_id"), defaultOrgID)
		cycleStart, _ := quota.CurrentBillingCycle(time.Now())

		if usageStore == nil {
			response.JSON(w, http.StatusOK, map[string]any{"data": []store.DailyUsage{}})
			return
		}

		daily, err := usageStore.GetDailyUsage(r.Context(), orgID, cycleStart)
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to retrieve daily usage")
			return
		}

		response.JSON(w, http.StatusOK, map[string]any{"data": daily})
	}
}

// EndpointUsageHandler handles GET /api/v1/usage/endpoints.
func EndpointUsageHandler(usageStore store.UsageStore, defaultOrgID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := resolveOrgID(r, r.URL.Query().Get("org_id"), defaultOrgID)
		cycleStart, _ := quota.CurrentBillingCycle(time.Now())

		if usageStore == nil {
			response.JSON(w, http.StatusOK, map[string]any{"data": []store.EndpointUsage{}})
			return
		}

		endpoints, err := usageStore.GetEndpointUsage(r.Context(), orgID, cycleStart)
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to retrieve endpoint usage")
			return
		}

		response.JSON(w, http.StatusOK, map[string]any{"data": endpoints})
	}
}

// UsageRecordsHandler handles GET /api/v1/usage/records.
func UsageRecordsHandler(usageStore store.UsageStore, defaultOrgID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := resolveOrgID(r, r.URL.Query().Get("org_id"), defaultOrgID)

		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 50
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if offset < 0 {
			offset = 0
		}
		statusCode, _ := strconv.Atoi(r.URL.Query().Get("status_code"))
		endpoint := strings.TrimSpace(r.URL.Query().Get("endpoint"))

		filter := store.UsageRecordFilter{
			Limit:      limit,
			Offset:     offset,
			StatusCode: statusCode,
			Endpoint:   endpoint,
		}

		if usageStore == nil {
			response.JSON(w, http.StatusOK, map[string]any{
				"data":   []store.UsageRecord{},
				"total":  0,
				"limit":  limit,
				"offset": offset,
			})
			return
		}

		records, total, err := usageStore.GetUsageRecords(r.Context(), orgID, filter)
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to retrieve usage records")
			return
		}

		response.JSON(w, http.StatusOK, map[string]any{
			"data":   records,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

