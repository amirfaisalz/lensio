variable "project" {
  type        = string
  description = "Project name identifier"
  default     = "nusaid"
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

variable "tenant_id" {
  type        = string
  description = "Azure AD tenant ID"
}

variable "private_endpoints_subnet_id" {
  type        = string
  description = "Subnet ID for Key Vault private endpoint"
  default     = null
}

variable "vnet_id" {
  type        = string
  description = "Virtual network ID for private DNS zone link"
  default     = null
}

variable "container_app_principal_id" {
  type        = string
  description = "Principal ID of Container App User Assigned Identity for RBAC assignment"
  default     = null
}

variable "db_admin_password" {
  type        = string
  description = "PostgreSQL admin password (auto-generated if empty)"
  default     = ""
  sensitive   = true
}

variable "database_url" {
  type        = string
  description = "Full PostgreSQL connection URL (can be constructed or set)"
  default     = ""
  sensitive   = true
}

variable "gemini_api_key" {
  type        = string
  description = "Google Gemini API key for OCR engine"
  default     = ""
  sensitive   = true
}

variable "jwt_secret" {
  type        = string
  description = "JWT signing secret"
  default     = ""
  sensitive   = true
}

variable "tags" {
  type        = map(string)
  description = "Resource tags"
  default     = {}
}
