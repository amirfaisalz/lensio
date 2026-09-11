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
  default     = "southeastasia"
}

variable "vnet_address_space" {
  type        = list(string)
  description = "Address space for the Virtual Network"
  default     = ["10.0.0.0/16"]
}

variable "container_apps_subnet_cidr" {
  type        = string
  description = "Subnet CIDR for Azure Container Apps Environment"
  default     = "10.0.0.0/23"
}

variable "postgres_subnet_cidr" {
  type        = string
  description = "Subnet CIDR for Azure Database for PostgreSQL Flexible Server"
  default     = "10.0.4.0/24"
}

variable "private_endpoints_subnet_cidr" {
  type        = string
  description = "Subnet CIDR for Private Endpoints (Key Vault, etc.)"
  default     = "10.0.5.0/24"
}

variable "tags" {
  type        = map(string)
  description = "Resource tags"
  default     = {}
}
