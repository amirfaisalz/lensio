output "server_id" {
  description = "ID of the PostgreSQL Flexible Server"
  value       = azurerm_postgresql_flexible_server.postgres.id
}

output "server_name" {
  description = "Name of the PostgreSQL Flexible Server"
  value       = azurerm_postgresql_flexible_server.postgres.name
}

output "server_fqdn" {
  description = "FQDN of the PostgreSQL Flexible Server"
  value       = azurerm_postgresql_flexible_server.postgres.fqdn
}

output "database_name" {
  description = "Application database name"
  value       = azurerm_postgresql_flexible_server_database.app_db.name
}

output "admin_username" {
  description = "Administrator login name"
  value       = var.admin_username
}

output "connection_string" {
  description = "Full connection URL for the application"
  value       = "postgres://${var.admin_username}:${random_password.admin.result}@${azurerm_postgresql_flexible_server.postgres.fqdn}:5432/${var.database_name}?sslmode=require"
  sensitive   = true
}
