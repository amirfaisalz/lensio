variable "project" {
  type        = string
  description = "Project name identifier"
  default     = "lensio"
}

variable "environment" {
  type        = string
  description = "Deployment environment (e.g. staging, production)"
}

variable "location" {
  type        = string
  description = "Azure region for resource deployment"
}

variable "resource_group_name" {
  type        = string
  description = "Name of the resource group"
}

variable "infrastructure_subnet_id" {
  type        = string
  description = "Subnet ID delegated to Microsoft.App/environments"
}

variable "api_image" {
  type        = string
  description = "Docker image for Lensio API"
  # Bootstrap only: after creation CD owns the image (lifecycle.ignore_changes).
  default = "ghcr.io/amirfaisalz/lensio/lensio-api:latest"
}

variable "dashboard_image" {
  type        = string
  description = "Docker image for Lensio Dashboard"
  # Bootstrap only: after creation CD owns the image (lifecycle.ignore_changes).
  default = "ghcr.io/amirfaisalz/lensio/lensio-dashboard:latest"
}

variable "database_url" {
  type        = string
  description = "PostgreSQL connection string"
  sensitive   = true
}

variable "gemini_api_key" {
  type        = string
  description = "Google Gemini API key for OCR engine"
  default     = ""
  sensitive   = true
}

variable "log_level" {
  type        = string
  description = "Application log level"
  default     = "info"
}

variable "api_cpu" {
  type        = number
  description = "CPU cores allocated to API container"
  default     = 0.5
}

variable "api_memory" {
  type        = string
  description = "Memory allocated to API container"
  default     = "1.0Gi"
}

variable "api_min_replicas" {
  type        = number
  description = "Minimum replicas for API"
  default     = 1
}

variable "api_max_replicas" {
  type        = number
  description = "Maximum replicas for API"
  default     = 3
}

variable "dashboard_cpu" {
  type        = number
  description = "CPU cores allocated to Dashboard container"
  default     = 0.25
}

variable "dashboard_memory" {
  type        = string
  description = "Memory allocated to Dashboard container"
  default     = "0.5Gi"
}

variable "dashboard_min_replicas" {
  type        = number
  description = "Minimum replicas for Dashboard"
  default     = 1
}

variable "dashboard_max_replicas" {
  type        = number
  description = "Maximum replicas for Dashboard"
  default     = 2
}

variable "tags" {
  type        = map(string)
  description = "Resource tags"
  default     = {}
}

variable "registry_username" {
  description = "GHCR user for pulling private images."
  type        = string
  default     = ""
}

variable "registry_password" {
  description = "GHCR token with read:packages. Empty means the images are public and no registry credentials are configured."
  type        = string
  sensitive   = true
  default     = ""
}

variable "ocr_provider" {
  description = "OCR engine to use. Anything other than a gemini_* value falls back to the deterministic mock engine."
  type        = string
  default     = "gemini_flash"

  validation {
    condition     = contains(["gemini_flash", "gemini", "gemini-flash", "mock"], var.ocr_provider)
    error_message = "ocr_provider must be one of: gemini_flash, gemini, gemini-flash, mock."
  }
}

variable "cors_allowed_origins" {
  description = "Extra browser origins permitted to call the API with credentials (e.g. a custom dashboard domain). The dashboard's Azure origin is always included."
  type        = list(string)
  default     = []
}

variable "smtp_host" {
  description = "SMTP host for verification and password-reset mail. The API refuses to start in production/staging without it."
  type        = string
  default     = ""
}

variable "smtp_port" {
  description = "SMTP submission port."
  type        = string
  default     = "587"
}

variable "smtp_username" {
  description = "SMTP username. Empty means unauthenticated relay."
  type        = string
  default     = ""
}

variable "smtp_password" {
  description = "SMTP password, stored as a container app secret."
  type        = string
  sensitive   = true
  default     = ""
}

variable "smtp_from" {
  description = "From address on transactional email."
  type        = string
  default     = "noreply@lensio.dev"
}

variable "api_public_url" {
  description = "Public API origin the dashboard calls (e.g. a custom domain). Empty uses the API's Azure origin."
  type        = string
  default     = ""
}

variable "app_base_url" {
  description = "Dashboard origin used to build the links inside transactional email. Empty uses the dashboard's Azure origin."
  type        = string
  default     = ""
}
