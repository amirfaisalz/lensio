variable "project" {
  type        = string
  description = "Project name identifier"
  default     = "lensio"
}

variable "environment" {
  type        = string
  description = "Deployment environment"

  validation {
    condition     = contains(["staging", "production"], var.environment)
    error_message = "environment must be staging or production."
  }
}

variable "location" {
  type        = string
  description = "Azure region for deployment"
  default     = "southeastasia"
}

variable "cloudflare_zone_id" {
  type        = string
  description = "Cloudflare DNS Zone ID. Empty skips DNS, TLS and the custom domain binding."
  default     = ""
}

variable "manage_cloudflare_zone" {
  type        = bool
  description = "Own the zone-wide Cloudflare settings and WAF rulesets. Exactly one environment sharing the zone sets this."
  default     = false
}

variable "cloudflare_proxied" {
  type        = bool
  description = "Proxy the public hostnames through Cloudflare."
  default     = true
}

variable "postgres_sku" {
  type        = string
  description = "PostgreSQL Flexible Server SKU"
}

variable "postgres_storage_mb" {
  type        = number
  description = "PostgreSQL storage in MB"
}

variable "postgres_ha_mode" {
  type        = string
  description = "PostgreSQL high availability mode"
}

variable "postgres_backup_retention_days" {
  type        = number
  description = "Backup retention days"
}

variable "postgres_geo_redundant_backups" {
  type        = bool
  description = "Enable geo-redundant backups"
  default     = false
}

variable "api_cpu" {
  type        = number
  description = "CPU allocated to API"
}

variable "api_memory" {
  type        = string
  description = "Memory allocated to API"
}

variable "api_min_replicas" {
  type        = number
  description = "Min API replicas"
}

variable "api_max_replicas" {
  type        = number
  description = "Max API replicas"
}

variable "dashboard_cpu" {
  type        = number
  description = "CPU allocated to Dashboard"
}

variable "dashboard_memory" {
  type        = string
  description = "Memory allocated to Dashboard"
}

variable "dashboard_min_replicas" {
  type        = number
  description = "Min Dashboard replicas"
}

variable "dashboard_max_replicas" {
  type        = number
  description = "Max Dashboard replicas"
}

variable "api_subdomain" {
  type        = string
  description = "API subdomain name"
}

variable "dashboard_subdomain" {
  type        = string
  description = "Dashboard subdomain name"
}

variable "tags" {
  type        = map(string)
  description = "Resource tags"
  default     = {}
}

variable "gemini_api_key" {
  type        = string
  description = "Google AI Studio key for the Gemini vision engine. Empty makes the API fall back to the deterministic mock engine."
  sensitive   = true
  default     = ""
}

variable "ocr_provider" {
  type        = string
  description = "OCR engine selector. Anything outside the gemini_* values serves deterministic fixture data."
  default     = "gemini_flash"
}

variable "cors_allowed_origins" {
  type        = list(string)
  description = "Extra browser origins allowed to call the API with credentials, e.g. a custom dashboard domain. The dashboard's Azure origin is always allowed."
  default     = []
}

variable "smtp_host" {
  type        = string
  description = "SMTP host for transactional email. Required: the API refuses to start without it in production/staging."
  default     = ""
}

variable "smtp_username" {
  type        = string
  description = "SMTP username."
  default     = ""
}

variable "smtp_password" {
  type        = string
  description = "SMTP password. Supply via TF_VAR_smtp_password."
  sensitive   = true
  default     = ""
}

variable "smtp_from" {
  type        = string
  description = "From address on transactional email."
  default     = "noreply@lensio.dev"
}

variable "api_public_url" {
  type        = string
  description = "Public API origin the dashboard calls, e.g. a custom domain. Empty uses the API's Azure origin."
  default     = ""
}

variable "app_base_url" {
  type        = string
  description = "Dashboard origin used to build links inside transactional email. Empty uses the dashboard's Azure origin."
  default     = ""
}

variable "registry_username" {
  type        = string
  description = "GHCR user for pulling private images."
  default     = ""
}

variable "registry_password" {
  type        = string
  description = "GHCR token with read:packages. Supply via TF_VAR_registry_password; empty assumes public images."
  sensitive   = true
  default     = ""
}
