terraform {
  required_version = ">= 1.9.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.116"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

locals {
  name_prefix = "${var.project}-${var.environment}"
  # An app's Azure FQDN is <app name>.<environment default domain>. Deriving both
  # origins from the environment avoids a cycle: the API needs the dashboard's
  # origin (CORS) and the dashboard needs the API's (API_URL).
  api_origin       = "https://ca-api-${local.name_prefix}.${azurerm_container_app_environment.cae.default_domain}"
  dashboard_origin = "https://ca-dash-${local.name_prefix}.${azurerm_container_app_environment.cae.default_domain}"
  common_tags = merge(var.tags, {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "OpenTofu"
  })
}

resource "azurerm_log_analytics_workspace" "logs" {
  name                = "log-${local.name_prefix}"
  location            = var.location
  resource_group_name = var.resource_group_name
  sku                 = "PerGB2018"
  retention_in_days   = 30
  tags                = local.common_tags
}

# Generated here rather than passed in: nothing outside the API needs the session
# key, and random_password keeps it stable across applies.
resource "random_password" "session_secret" {
  length  = 64
  special = false
}

# Bearer token Prometheus presents on /metrics; the API refuses to start in
# production/staging without one because the endpoint sits on the public ingress.
resource "random_password" "metrics_token" {
  length  = 48
  special = false
}

resource "azurerm_container_app_environment" "cae" {
  name                           = "cae-${local.name_prefix}"
  location                       = var.location
  resource_group_name            = var.resource_group_name
  log_analytics_workspace_id     = azurerm_log_analytics_workspace.logs.id
  infrastructure_subnet_id       = var.infrastructure_subnet_id
  internal_load_balancer_enabled = false
  tags                           = local.common_tags
}

# Go API Container App
resource "azurerm_container_app" "api" {
  name                         = "ca-api-${local.name_prefix}"
  container_app_environment_id = azurerm_container_app_environment.cae.id
  resource_group_name          = var.resource_group_name
  # Multiple keeps the previous revision addressable so a rollback is a traffic
  # shift, not a rebuild (scripts/rollback.sh).
  revision_mode = "Multiple"
  tags          = local.common_tags

  # CD (scripts/deploy.sh) owns the running image and the traffic split; without
  # this every apply would revert a deploy or undo a rollback. The custom domain
  # binding is owned by azurerm_container_app_custom_domain.
  lifecycle {
    ignore_changes = [template[0].container[0].image, ingress[0].traffic_weight, ingress[0].custom_domain]
  }

  dynamic "registry" {
    for_each = nonsensitive(var.registry_password != "") ? toset(["ghcr"]) : toset([])
    content {
      server               = "ghcr.io"
      username             = var.registry_username
      password_secret_name = "registry-password" # gitleaks:allow -- names the Container Apps secret, not a password
    }
  }

  dynamic "secret" {
    for_each = nonsensitive(var.registry_password != "") ? toset(["ghcr"]) : toset([])
    content {
      name  = "registry-password"
      value = var.registry_password
    }
  }

  ingress {
    external_enabled = true
    target_port      = 8080
    transport        = "auto"

    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }

  secret {
    name  = "database-url"
    value = var.database_url
  }

  # The API refuses to start in production/staging without SESSION_SECRET, so it
  # must be provisioned here rather than set by hand on the running revision.
  secret {
    name  = "session-secret"
    value = random_password.session_secret.result
  }

  secret {
    name  = "metrics-token"
    value = random_password.metrics_token.result
  }

  # The API also refuses to start in production/staging without an SMTP host:
  # unsent verification mail means no self-service account can ever be used.
  dynamic "secret" {
    for_each = nonsensitive(var.smtp_password != "") ? toset(["smtp"]) : toset([])
    content {
      name  = "smtp-password"
      value = var.smtp_password
    }
  }

  dynamic "secret" {
    # nonsensitive(): gemini_api_key is a sensitive variable, so the comparison
    # result inherits that mark and for_each refuses marked values (a key would
    # leak it). Whether a key was supplied is not itself a secret.
    for_each = nonsensitive(var.gemini_api_key != "") ? toset(["gemini"]) : toset([])
    content {
      name  = "gemini-api-key"
      value = var.gemini_api_key
    }
  }

  template {
    min_replicas = var.api_min_replicas
    max_replicas = var.api_max_replicas

    container {
      name   = "api"
      image  = var.api_image
      cpu    = var.api_cpu
      memory = var.api_memory

      env {
        name  = "ENV"
        value = var.environment
      }
      env {
        name  = "PORT"
        value = "8080"
      }
      env {
        name  = "LOG_LEVEL"
        value = var.log_level
      }
      # Without this the provider silently falls back to the deterministic mock
      # engine, which would serve fixture data to paying callers.
      env {
        name  = "OCR_PROVIDER"
        value = var.ocr_provider
      }
      # Unset means no CORS headers at all, which breaks the dashboard because it
      # is served from a different origin than the API.
      env {
        name  = "CORS_ALLOWED_ORIGINS"
        value = join(",", distinct(concat([local.dashboard_origin], var.cors_allowed_origins)))
      }
      env {
        name        = "DATABASE_URL"
        secret_name = "database-url"
      }
      env {
        name        = "SESSION_SECRET"
        secret_name = "session-secret"
      }
      env {
        name        = "METRICS_TOKEN"
        secret_name = "metrics-token"
      }

      env {
        name  = "SMTP_HOST"
        value = var.smtp_host
      }
      env {
        name  = "SMTP_PORT"
        value = var.smtp_port
      }
      env {
        name  = "SMTP_USERNAME"
        value = var.smtp_username
      }
      env {
        name  = "SMTP_FROM"
        value = var.smtp_from
      }
      env {
        name  = "APP_BASE_URL"
        value = var.app_base_url != "" ? var.app_base_url : local.dashboard_origin
      }

      dynamic "env" {
        for_each = nonsensitive(var.smtp_password != "") ? toset(["smtp"]) : toset([])
        content {
          name        = "SMTP_PASSWORD"
          secret_name = "smtp-password"
        }
      }

      dynamic "env" {
        # nonsensitive(): see the secret block above.
        for_each = nonsensitive(var.gemini_api_key != "") ? toset(["gemini"]) : toset([])
        content {
          name        = "GEMINI_API_KEY"
          secret_name = "gemini-api-key"
        }
      }

      liveness_probe {
        transport               = "HTTP"
        port                    = 8080
        path                    = "/health"
        interval_seconds        = 10
        timeout                 = 3
        failure_count_threshold = 3
      }

      readiness_probe {
        transport               = "HTTP"
        port                    = 8080
        path                    = "/ready"
        interval_seconds        = 10
        timeout                 = 3
        success_count_threshold = 1
        failure_count_threshold = 3
      }

      startup_probe {
        transport               = "HTTP"
        port                    = 8080
        path                    = "/health"
        interval_seconds        = 5
        timeout                 = 3
        failure_count_threshold = 5
      }
    }

    http_scale_rule {
      name                = "http-concurrency-rule"
      concurrent_requests = "50"
    }
  }
}

# React Dashboard Container App
resource "azurerm_container_app" "dashboard" {
  name                         = "ca-dash-${local.name_prefix}"
  container_app_environment_id = azurerm_container_app_environment.cae.id
  resource_group_name          = var.resource_group_name
  revision_mode                = "Multiple"
  tags                         = local.common_tags

  # See the API app: CD owns image and traffic.
  lifecycle {
    ignore_changes = [template[0].container[0].image, ingress[0].traffic_weight, ingress[0].custom_domain]
  }

  dynamic "registry" {
    for_each = nonsensitive(var.registry_password != "") ? toset(["ghcr"]) : toset([])
    content {
      server               = "ghcr.io"
      username             = var.registry_username
      password_secret_name = "registry-password" # gitleaks:allow -- names the Container Apps secret, not a password
    }
  }

  dynamic "secret" {
    for_each = nonsensitive(var.registry_password != "") ? toset(["ghcr"]) : toset([])
    content {
      name  = "registry-password"
      value = var.registry_password
    }
  }

  ingress {
    external_enabled = true
    target_port      = 80
    transport        = "auto"

    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }

  template {
    min_replicas = var.dashboard_min_replicas
    max_replicas = var.dashboard_max_replicas

    container {
      name   = "dashboard"
      image  = var.dashboard_image
      cpu    = var.dashboard_cpu
      memory = var.dashboard_memory

      # Written into /config.js at container start (40-lensio-config.sh), so the
      # same image is promoted from staging to production unchanged.
      env {
        name  = "API_URL"
        value = var.api_public_url != "" ? var.api_public_url : local.api_origin
      }

      liveness_probe {
        transport               = "HTTP"
        port                    = 80
        path                    = "/healthz"
        interval_seconds        = 10
        timeout                 = 3
        failure_count_threshold = 3
      }

      readiness_probe {
        transport               = "HTTP"
        port                    = 80
        path                    = "/healthz"
        interval_seconds        = 10
        timeout                 = 3
        success_count_threshold = 1
        failure_count_threshold = 3
      }
    }
  }
}
