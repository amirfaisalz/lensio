terraform {
  required_version = ">= 1.8.0"
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.30"
    }
  }
}

# CNAME DNS Record for API
resource "cloudflare_record" "api" {
  zone_id = var.zone_id
  name    = var.api_subdomain
  content = var.api_target_fqdn
  type    = "CNAME"
  proxied = true
  ttl     = 1 # Automatic when proxied
  comment = "Managed by OpenTofu - Lensio API (${var.environment})"
}

# CNAME DNS Record for Dashboard
resource "cloudflare_record" "dashboard" {
  zone_id = var.zone_id
  name    = var.dashboard_subdomain
  content = var.dashboard_target_fqdn
  type    = "CNAME"
  proxied = true
  ttl     = 1 # Automatic when proxied
  comment = "Managed by OpenTofu - Lensio Dashboard (${var.environment})"
}

# Zone Settings & SSL/TLS Configuration
resource "cloudflare_zone_settings_override" "settings" {
  zone_id = var.zone_id

  settings {
    ssl                      = "strict"
    always_use_https         = "on"
    min_tls_version          = "1.2"
    tls_1_3                  = "on"
    brotli                   = "on"
    early_hints              = "on"
    browser_check            = "on"
    security_level           = "medium"
    challenge_ttl            = 1800
    automatic_https_rewrites = "on"
  }
}

# WAF Edge Rate Limiting Ruleset for OCR Endpoints
resource "cloudflare_ruleset" "ocr_rate_limiting" {
  count       = var.enable_waf ? 1 : 0
  zone_id     = var.zone_id
  name        = "lensio-${var.environment}-ocr-rate-limit"
  description = "Edge rate limiting protection for Lensio OCR API"
  kind        = "zone"
  phase       = "http_ratelimit"

  rules {
    action      = "block"
    description = "Limit excessive OCR extraction requests per IP"
    expression  = "(http.request.uri.path contains \"/api/v1/ocr/\")"
    enabled     = true

    ratelimit {
      characteristics     = ["ip.src"]
      period              = 60
      requests_per_period = var.rate_limit_requests_per_minute
      mitigation_timeout  = 60
    }
  }
}

# Custom WAF Ruleset to block malicious payloads and bad user agents
resource "cloudflare_ruleset" "custom_waf" {
  count       = var.enable_waf ? 1 : 0
  zone_id     = var.zone_id
  name        = "lensio-${var.environment}-custom-waf"
  description = "Custom WAF rules blocking suspicious traffic"
  kind        = "zone"
  phase       = "http_request_firewall_custom"

  rules {
    action      = "block"
    description = "Block common automated scanner user agents"
    expression  = "(http.user_agent contains \"sqlmap\" or http.user_agent contains \"nikto\" or http.user_agent contains \"acunetix\")"
    enabled     = true
  }
}
