mock_provider "cloudflare" {}

variables {
  zone_id               = "023e105f4ecef8ad9ca31a8372d0c353"
  environment           = "staging"
  api_subdomain         = "staging-api"
  dashboard_subdomain   = "staging-dashboard"
  api_target_fqdn       = "ca-api-staging.southeastasia.azurecontainerapps.io"
  dashboard_target_fqdn = "ca-dash-staging.southeastasia.azurecontainerapps.io"
}

run "validate_cloudflare_configuration" {
  command = plan

  assert {
    condition     = cloudflare_record.api.proxied == true
    error_message = "API DNS record must be proxied through Cloudflare"
  }

  assert {
    condition     = cloudflare_record.dashboard.proxied == true
    error_message = "Dashboard DNS record must be proxied through Cloudflare"
  }

  assert {
    condition     = cloudflare_zone_settings_override.settings.settings[0].ssl == "strict"
    error_message = "Cloudflare SSL mode must be strict"
  }
}
