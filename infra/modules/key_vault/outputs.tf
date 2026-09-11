output "key_vault_id" {
  description = "ID of the Azure Key Vault"
  value       = azurerm_key_vault.kv.id
}

output "key_vault_name" {
  description = "Name of the Azure Key Vault"
  value       = azurerm_key_vault.kv.name
}

output "key_vault_uri" {
  description = "Vault URI of the Azure Key Vault"
  value       = azurerm_key_vault.kv.vault_uri
}

output "db_admin_password" {
  description = "Resolved database admin password"
  value       = local.resolved_db_password
  sensitive   = true
}

output "db_password_secret_id" {
  description = "Key Vault Secret ID for database-password"
  value       = azurerm_key_vault_secret.db_password.id
}

output "db_password_secret_versionless_id" {
  description = "Versionless Key Vault Secret ID for database-password"
  value       = azurerm_key_vault_secret.db_password.versionless_id
}

output "jwt_secret" {
  description = "Resolved JWT secret"
  value       = local.resolved_jwt_secret
  sensitive   = true
}
