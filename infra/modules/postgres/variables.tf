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

variable "vnet_id" {
  type        = string
  description = "Virtual Network ID for Private DNS zone link"
}

variable "delegated_subnet_id" {
  type        = string
  description = "Subnet ID delegated to Microsoft.DBforPostgreSQL/flexibleServers"
}

variable "admin_username" {
  type        = string
  description = "PostgreSQL administrator login name"
  default     = "lensioadmin"
}

variable "admin_password" {
  type        = string
  description = "PostgreSQL administrator password"
  sensitive   = true
}

variable "postgres_version" {
  type        = string
  description = "PostgreSQL major version"
  default     = "16"
}

variable "sku_name" {
  type        = string
  description = "PostgreSQL Flexible Server SKU tier and size"
  default     = "B_Standard_B1ms"
}

variable "storage_mb" {
  type        = number
  description = "Max storage allowed for the server in MB"
  default     = 32768
}

variable "backup_retention_days" {
  type        = number
  description = "Backup retention days (7 for staging, 35 for production)"
  default     = 7
}

variable "geo_redundant_backups_enabled" {
  type        = bool
  description = "Enable geo-redundant backups for disaster recovery"
  default     = false
}

variable "high_availability_mode" {
  type        = string
  description = "High availability mode (Disabled, ZoneRedundant, or SameZone)"
  default     = "Disabled"
}

variable "database_name" {
  type        = string
  description = "Default application database name"
  default     = "lensio"
}

variable "tags" {
  type        = map(string)
  description = "Resource tags"
  default     = {}
}
