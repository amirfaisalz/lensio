terraform {
  required_version = ">= 1.8.0"
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.30"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.116"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }
}

data "cloudflare_zone" "zone" {
  zone_id = var.zone_id
}

locals {
  api_hostname       = "${var.api_subdomain}.${data.cloudflare_zone.zone.name}"
  dashboard_hostname = "${var.dashboard_subdomain}.${data.cloudflare_zone.zone.name}"
}

# CNAME DNS Record for API. Targets the app's stable ingress FQDN; a revision
# FQDN would go dead on the next deploy.
resource "cloudflare_record" "api" {
  zone_id = var.zone_id
  name    = var.api_subdomain
  content = var.api_target_fqdn
  type    = "CNAME"
  proxied = var.proxied
  ttl     = 1 # Automatic when proxied
  comment = "Managed by OpenTofu - Lensio API (${var.environment})"
}

# CNAME DNS Record for Dashboard
resource "cloudflare_record" "dashboard" {
  zone_id = var.zone_id
  name    = var.dashboard_subdomain
  content = var.dashboard_target_fqdn
  type    = "CNAME"
  proxied = var.proxied
  ttl     = 1 # Automatic when proxied
  comment = "Managed by OpenTofu - Lensio Dashboard (${var.environment})"
}

# Container Apps proves domain ownership through asuid.<subdomain> TXT records.
resource "cloudflare_record" "api_verification" {
  zone_id = var.zone_id
  name    = "asuid.${var.api_subdomain}"
  content = var.api_verification_id
  type    = "TXT"
  ttl     = 300
}

resource "cloudflare_record" "dashboard_verification" {
  zone_id = var.zone_id
  name    = "asuid.${var.dashboard_subdomain}"
  content = var.dashboard_verification_id
  type    = "TXT"
  ttl     = 300
}

# Cloudflare Origin CA certificate for the origin. Full (strict) SSL requires the
# origin to present a certificate Cloudflare trusts for the requested hostname;
# the default *.azurecontainerapps.io certificate does not cover it.
resource "tls_private_key" "origin" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "origin" {
  private_key_pem = tls_private_key.origin.private_key_pem
  dns_names       = [local.api_hostname, local.dashboard_hostname]

  subject {
    common_name = local.api_hostname
  }
}

resource "cloudflare_origin_ca_certificate" "origin" {
  csr                = tls_cert_request.origin.cert_request_pem
  hostnames          = [local.api_hostname, local.dashboard_hostname]
  request_type       = "origin-rsa"
  requested_validity = 5475
}

resource "azurerm_container_app_environment_certificate" "origin" {
  name                         = "cf-origin-${var.environment}"
  container_app_environment_id = var.container_app_environment_id
  certificate_blob_base64      = base64encode("${cloudflare_origin_ca_certificate.origin.certificate}${tls_private_key.origin.private_key_pem}")
  certificate_password         = ""
}

# Without a binding, Container Apps routes on the Host header and answers 404 for
# a hostname it does not know.
resource "azurerm_container_app_custom_domain" "api" {
  name                                     = local.api_hostname
  container_app_id                         = var.api_app_id
  container_app_environment_certificate_id = azurerm_container_app_environment_certificate.origin.id
  certificate_binding_type                 = "SniEnabled"

  depends_on = [cloudflare_record.api, cloudflare_record.api_verification]
}

resource "azurerm_container_app_custom_domain" "dashboard" {
  name                                     = local.dashboard_hostname
  container_app_id                         = var.dashboard_app_id
  container_app_environment_certificate_id = azurerm_container_app_environment_certificate.origin.id
  certificate_binding_type                 = "SniEnabled"

  depends_on = [cloudflare_record.dashboard, cloudflare_record.dashboard_verification]
}

# Everything below is zone-wide. Staging and production share one zone, so only
# the stack with manage_zone = true owns it; two owners would overwrite each other,
# and Cloudflare allows only one zone ruleset per phase.
resource "cloudflare_zone_settings_override" "settings" {
  count   = var.manage_zone ? 1 : 0
  zone_id = var.zone_id

  settings {
    ssl                      = "strict"
    always_use_https         = "on"
    min_tls_version          = "1.2"
    tls_1_3                  = "on"
    brotli                   = "on"
    early_hints              = "on"
    browser_check            = "on"
    security_level           = "medium"
    challenge_ttl            = 1800
    automatic_https_rewrites = "on"
  }
}

# WAF Edge Rate Limiting Ruleset for OCR Endpoints (every environment's hostname)
resource "cloudflare_ruleset" "ocr_rate_limiting" {
  count       = var.manage_zone ? 1 : 0
  zone_id     = var.zone_id
  name        = "lensio-ocr-rate-limit"
  description = "Edge rate limiting protection for Lensio OCR API"
  kind        = "zone"
  phase       = "http_ratelimit"

  rules {
    action      = "block"
    description = "Limit excessive OCR extraction requests per IP"
    expression  = "(http.request.uri.path contains \"/api/v1/ocr/\")"
    enabled     = true

    ratelimit {
      # Cloudflare counts per data centre and requires cf.colo.id in the key.
      characteristics     = ["ip.src", "cf.colo.id"]
      period              = 60
      requests_per_period = var.rate_limit_requests_per_minute
      mitigation_timeout  = 60
    }
  }
}

# Custom WAF Ruleset to block malicious payloads and bad user agents
resource "cloudflare_ruleset" "custom_waf" {
  count       = var.manage_zone ? 1 : 0
  zone_id     = var.zone_id
  name        = "lensio-custom-waf"
  description = "Custom WAF rules blocking suspicious traffic"
  kind        = "zone"
  phase       = "http_request_firewall_custom"

  rules {
    action      = "block"
    description = "Block common automated scanner user agents"
    expression  = "(http.user_agent contains \"sqlmap\" or http.user_agent contains \"nikto\" or http.user_agent contains \"acunetix\")"
    enabled     = true
  }
}
