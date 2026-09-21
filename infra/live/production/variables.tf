variable "project" {
  type        = string
  description = "Project name identifier"
  default     = "lensio"
}

variable "environment" {
  type        = string
  description = "Deployment environment"
  default     = "production"
}

variable "location" {
  type        = string
  description = "Azure region for deployment"
  default     = "southeastasia"
}

variable "tenant_id" {
  type        = string
  description = "Azure AD tenant ID"
  default     = "00000000-0000-0000-0000-000000000000"
}

variable "cloudflare_zone_id" {
  type        = string
  description = "Cloudflare DNS Zone ID"
  default     = ""
}

variable "postgres_sku" {
  type        = string
  description = "PostgreSQL Flexible Server SKU"
  default     = "GP_Standard_D2ds_v5"
}

variable "postgres_storage_mb" {
  type        = number
  description = "PostgreSQL storage in MB"
  default     = 65536
}

variable "postgres_ha_mode" {
  type        = string
  description = "PostgreSQL high availability mode"
  default     = "ZoneRedundant"
}

variable "postgres_backup_retention_days" {
  type        = number
  description = "Backup retention days"
  default     = 35
}

variable "postgres_geo_redundant_backups" {
  type        = bool
  description = "Enable geo-redundant backups"
  default     = true
}

variable "api_image" {
  type        = string
  description = "Docker image for Lensio API"
  default     = "ghcr.io/amirfaisalz/lensio-api:1.0.0"
}

variable "dashboard_image" {
  type        = string
  description = "Docker image for Lensio Dashboard"
  default     = "ghcr.io/amirfaisalz/lensio-dashboard:1.0.0"
}

variable "api_cpu" {
  type        = number
  description = "CPU allocated to API"
  default     = 1.0
}

variable "api_memory" {
  type        = string
  description = "Memory allocated to API"
  default     = "2.0Gi"
}

variable "api_min_replicas" {
  type        = number
  description = "Min API replicas"
  default     = 2
}

variable "api_max_replicas" {
  type        = number
  description = "Max API replicas"
  default     = 10
}

variable "dashboard_cpu" {
  type        = number
  description = "CPU allocated to Dashboard"
  default     = 0.5
}

variable "dashboard_memory" {
  type        = string
  description = "Memory allocated to Dashboard"
  default     = "1.0Gi"
}

variable "dashboard_min_replicas" {
  type        = number
  description = "Min Dashboard replicas"
  default     = 2
}

variable "dashboard_max_replicas" {
  type        = number
  description = "Max Dashboard replicas"
  default     = 5
}

variable "api_subdomain" {
  type        = string
  description = "API subdomain name"
  default     = "api"
}

variable "dashboard_subdomain" {
  type        = string
  description = "Dashboard subdomain name"
  default     = "dashboard"
}

variable "tags" {
  type        = map(string)
  description = "Resource tags"
  default     = {}
}

variable "session_secret" {
  type        = string
  description = "HMAC signing key for session cookies and the idempotency response sealer. Supply via TF_VAR_session_secret or Key Vault; the API refuses to start without it."
  sensitive   = true
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
  description = "Browser origins allowed to call the API with credentials, i.e. the dashboard origin. Empty emits no CORS headers and breaks the dashboard."
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
  description = "SMTP password. Supply via TF_VAR_smtp_password or Key Vault."
  sensitive   = true
  default     = ""
}

variable "smtp_from" {
  type        = string
  description = "From address on transactional email."
  default     = "noreply@lensio.dev"
}

variable "app_base_url" {
  type        = string
  description = "Dashboard origin used to build links inside transactional email."
  default     = "https://dashboard.lensio.dev"
}
