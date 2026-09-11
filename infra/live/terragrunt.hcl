# Root Terragrunt Configuration
# Applies to all environments (staging, production)

locals {
  # Load environment-level variables from env.hcl if present
  env_vars = fileexists("${get_terragrunt_dir()}/env.hcl") ? read_terragrunt_config("${get_terragrunt_dir()}/env.hcl").locals : {}

  project     = "nusaid"
  environment = lookup(local.env_vars, "environment", "staging")
  location    = lookup(local.env_vars, "location", "southeastasia")

  tags = {
    Project     = local.project
    Environment = local.environment
    ManagedBy   = "Terragrunt/OpenTofu"
  }
}

# Generate OpenTofu provider configurations
generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
terraform {
  required_version = ">= 1.8.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.116"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.30"
    }
  }
}

provider "azurerm" {
  features {
    key_vault {
      purge_soft_delete_on_destroy    = false
      recover_soft_deleted_key_vaults = true
    }
    resource_group {
      prevent_deletion_if_contains_resources = false
    }
  }
}

provider "cloudflare" {}
EOF
}

# Remote State Backend Configuration (Azure Blob Storage)
remote_state {
  backend = "azurerm"
  config = {
    resource_group_name  = "rg-nusaid-tfstate"
    storage_account_name = "stnusaidtfstate"
    container_name       = "tfstate"
    key                  = "${path_relative_to_include()}/terraform.tfstate"
  }
  generate = {
    path      = "backend.tf"
    if_exists = "overwrite_terragrunt"
  }
}

inputs = {
  project     = local.project
  environment = local.environment
  location    = local.location
  tags        = local.tags
}
