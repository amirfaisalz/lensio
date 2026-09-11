terraform {
  required_version = ">= 1.8.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.116"
    }
  }
}

locals {
  name_prefix = "${var.project}-${var.environment}"
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

resource "azurerm_user_assigned_identity" "ca_identity" {
  name                = "id-ca-${local.name_prefix}"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = local.common_tags
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
  revision_mode                = "Single"
  tags                         = local.common_tags

  identity {
    type         = "UserAssigned"
    identity_ids = [azurerm_user_assigned_identity.ca_identity.id]
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

  dynamic "secret" {
    for_each = var.gemini_api_key != "" ? [1] : []
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
      env {
        name  = "RATE_LIMIT_ENABLED"
        value = "true"
      }
      env {
        name        = "DATABASE_URL"
        secret_name = "database-url"
      }

      dynamic "env" {
        for_each = var.gemini_api_key != "" ? [1] : []
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
  revision_mode                = "Single"
  tags                         = local.common_tags

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
