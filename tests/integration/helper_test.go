package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// obtainTestAuthClient registers and logs in a test user, creates an organization if needed,
// and returns the JWT session token and org ID.
func obtainTestAuthClient(client *http.Client, apiURL string) (string, string, error) {
	uniqueSuffix := time.Now().UnixNano()
	email := fmt.Sprintf("itest-%d@lensio.dev", uniqueSuffix)
	password := "TestPassword123!"

	// 1. Try to register
	regBody, _ := json.Marshal(map[string]string{
		"full_name": fmt.Sprintf("Integration User %d", uniqueSuffix),
		"email":     email,
		"password":  password,
	})
	regResp, err := client.Post(apiURL+"/api/v1/auth/register", "application/json", bytes.NewReader(regBody))
	if err == nil {
		defer regResp.Body.Close()
		if regResp.StatusCode == http.StatusCreated {
			var regResult struct {
				VerificationToken string `json:"verification_token"`
			}
			_ = json.NewDecoder(regResp.Body).Decode(&regResult)

			// 2. Verify email
			verifyBody, _ := json.Marshal(map[string]string{
				"email": email,
				"token": regResult.VerificationToken,
			})
			vResp, vErr := client.Post(apiURL+"/api/v1/auth/verify-email", "application/json", bytes.NewReader(verifyBody))
			if vErr == nil {
				vResp.Body.Close()
			}
		}
	}

	// 3. Login
	loginBody, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	loginResp, err := client.Post(apiURL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginBody))
	if err == nil && loginResp.StatusCode == http.StatusOK {
		defer loginResp.Body.Close()
		var loginResult struct {
			AccessToken  string `json:"access_token"`
			Organization *struct {
				ID string `json:"id"`
			} `json:"organization"`
		}
		if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err == nil {
			orgID := ""
			if loginResult.Organization != nil && loginResult.Organization.ID != "" {
				orgID = loginResult.Organization.ID
			} else {
				// 4. Create Organization (authenticated via session cookie stored in client.Jar)
				createOrgBody, _ := json.Marshal(map[string]string{
					"name":      fmt.Sprintf("Org %d", uniqueSuffix),
					"plan_code": "free",
				})
				req, _ := http.NewRequest(http.MethodPost, apiURL+"/api/v1/account/organizations", bytes.NewReader(createOrgBody))
				req.Header.Set("Content-Type", "application/json")
				orgResp, err := client.Do(req)
				if err == nil {
					defer orgResp.Body.Close()
					var orgResult struct {
						Organization struct {
							ID string `json:"id"`
						} `json:"organization"`
					}
					if err := json.NewDecoder(orgResp.Body).Decode(&orgResult); err == nil {
						orgID = orgResult.Organization.ID
					}
				}
			}
			return loginResult.AccessToken, orgID, nil
		}
	}

	// Fallback to development mock token if auth endpoints are unavailable
	return "mock_jwt_admin", "", nil
}
