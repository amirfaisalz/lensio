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

output "cloudflare_api_hostname" {
  description = "Cloudflare API Hostname"
  value       = length(module.cloudflare) > 0 ? module.cloudflare[0].api_hostname : null
}

output "cloudflare_dashboard_hostname" {
  description = "Cloudflare Dashboard Hostname"
  value       = length(module.cloudflare) > 0 ? module.cloudflare[0].dashboard_hostname : null
}

output "metrics_token" {
  description = "Bearer token for scraping /metrics (tofu output -raw metrics_token)"
  value       = module.container_apps.metrics_token
  sensitive   = true
}
