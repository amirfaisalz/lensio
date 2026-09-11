variable "zone_id" {
  type        = string
  description = "Cloudflare DNS Zone ID"
}

variable "environment" {
  type        = string
  description = "Deployment environment (e.g. staging, production)"
}

variable "api_subdomain" {
  type        = string
  description = "Subdomain prefix for API (e.g. api, staging-api)"
  default     = "api"
}

variable "dashboard_subdomain" {
  type        = string
  description = "Subdomain prefix for Dashboard (e.g. dashboard, staging-dashboard)"
  default     = "dashboard"
}

variable "api_target_fqdn" {
  type        = string
  description = "Target Azure Container App FQDN for API"
}

variable "dashboard_target_fqdn" {
  type        = string
  description = "Target Azure Container App FQDN for Dashboard"
}

variable "enable_waf" {
  type        = bool
  description = "Enable Cloudflare WAF custom rulesets and edge rate limiting"
  default     = true
}

variable "rate_limit_requests_per_minute" {
  type        = number
  description = "Max OCR requests per minute at Cloudflare edge"
  default     = 120
}
