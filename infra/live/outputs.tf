output "resource_group_name" {
  description = "Resource group name"
  value       = module.networking.resource_group_name
}

output "postgres_fqdn" {
  description = "PostgreSQL Flexible Server FQDN"
  value       = module.postgres.server_fqdn
}

output "api_fqdn" {
  description = "Azure Container App API FQDN"
  value       = module.container_apps.api_fqdn
}

output "dashboard_fqdn" {
  description = "Azure Container App Dashboard FQDN"
  value       = module.container_apps.dashboard_fqdn
}

output "metrics_token" {
  description = "Bearer token for scraping /metrics (tofu output -raw metrics_token)"
  value       = module.container_apps.metrics_token
  sensitive   = true
}
