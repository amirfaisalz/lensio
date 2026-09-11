output "api_hostname" {
  description = "Configured public hostname for the API"
  value       = cloudflare_record.api.hostname
}

output "dashboard_hostname" {
  description = "Configured public hostname for the Dashboard"
  value       = cloudflare_record.dashboard.hostname
}

output "zone_id" {
  description = "Target Cloudflare Zone ID"
  value       = var.zone_id
}
