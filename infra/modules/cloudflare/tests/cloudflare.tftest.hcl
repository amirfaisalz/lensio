mock_provider "cloudflare" {
  mock_data "cloudflare_zone" {
    defaults = {
      name = "lensio.dev"
    }
  }
}

mock_provider "azurerm" {
  mock_resource "azurerm_container_app_environment_certificate" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.App/managedEnvironments/cae-lensio-staging/certificates/cf-origin-staging"
    }
  }
}

variables {
  zone_id                      = "023e105f4ecef8ad9ca31a8372d0c353"
  environment                  = "staging"
  api_subdomain                = "staging-api"
  dashboard_subdomain          = "staging-dashboard"
  api_target_fqdn              = "ca-api-staging.southeastasia.azurecontainerapps.io"
  dashboard_target_fqdn        = "ca-dash-staging.southeastasia.azurecontainerapps.io"
  container_app_environment_id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.App/managedEnvironments/cae-lensio-staging"
  api_app_id                   = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.App/containerApps/ca-api-lensio-staging"
  dashboard_app_id             = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.App/containerApps/ca-dash-lensio-staging"
  api_verification_id          = "SYNTHETICVERIFICATIONID"
  dashboard_verification_id    = "SYNTHETICVERIFICATIONID"
}

run "validate_cloudflare_configuration" {
  command = plan

  assert {
    condition     = cloudflare_record.api.proxied == true && cloudflare_record.dashboard.proxied == true
    error_message = "DNS records must be proxied through Cloudflare by default"
  }

  # Container Apps routes on the Host header; without a binding it answers 404.
  assert {
    condition     = azurerm_container_app_custom_domain.api.name == "staging-api.lensio.dev" && azurerm_container_app_custom_domain.dashboard.name == "staging-dashboard.lensio.dev"
    error_message = "Each public hostname must be bound to its Container App"
  }

  assert {
    condition     = cloudflare_record.api_verification.name == "asuid.staging-api" && cloudflare_record.api_verification.type == "TXT"
    error_message = "Custom domain ownership needs the asuid TXT record"
  }

  assert {
    condition     = toset(cloudflare_origin_ca_certificate.origin.hostnames) == toset(["staging-api.lensio.dev", "staging-dashboard.lensio.dev"])
    error_message = "Origin certificate must cover both hostnames for Full (strict) SSL"
  }

  # A non-owner stack must not touch zone-wide state.
  assert {
    condition     = length(cloudflare_zone_settings_override.settings) == 0 && length(cloudflare_ruleset.ocr_rate_limiting) == 0 && length(cloudflare_ruleset.custom_waf) == 0
    error_message = "Zone-wide resources must only exist in the manage_zone stack"
  }
}

run "zone_owner" {
  command = plan

  variables {
    manage_zone = true
  }

  assert {
    condition     = cloudflare_zone_settings_override.settings[0].settings[0].ssl == "strict"
    error_message = "Cloudflare SSL mode must be strict"
  }

  assert {
    condition     = contains(cloudflare_ruleset.ocr_rate_limiting[0].rules[0].ratelimit[0].characteristics, "cf.colo.id")
    error_message = "Cloudflare rate limit characteristics must include cf.colo.id"
  }
}
