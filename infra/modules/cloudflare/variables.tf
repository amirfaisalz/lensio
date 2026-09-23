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

variable "manage_zone" {
  type        = bool
  description = "Own the zone-wide settings and WAF/rate-limit rulesets. Exactly one stack sharing the zone may set this."
  default     = false
}

variable "proxied" {
  type        = bool
  description = "Proxy the CNAMEs through Cloudflare. Set false for the first apply if Container Apps cannot validate the custom domain behind the proxy."
  default     = true
}

variable "container_app_environment_id" {
  type        = string
  description = "Container Apps environment that stores the origin certificate"
}

variable "api_app_id" {
  type        = string
  description = "API Container App ID the api hostname binds to"
}

variable "dashboard_app_id" {
  type        = string
  description = "Dashboard Container App ID the dashboard hostname binds to"
}

variable "api_verification_id" {
  type        = string
  description = "API app's custom_domain_verification_id, published as the asuid TXT record"
  sensitive   = true
}

variable "dashboard_verification_id" {
  type        = string
  description = "Dashboard app's custom_domain_verification_id, published as the asuid TXT record"
  sensitive   = true
}

variable "rate_limit_requests_per_minute" {
  type        = number
  description = "Max OCR requests per minute at Cloudflare edge"
  default     = 120
}
