# ADR-004: Azure Container Apps vs. Kubernetes (Deliberate Simplicity)

- **Status**: Accepted
- **Date**: 2026-09-12
- **Deciders**: NusaID Engineering Team
- **Technical Context**: `PRD Section 22`, `PRD Section 23`, `PRD Section 25`, `PRD Section 32`, `PRD Section 41`

---

## 1. Context and Problem Statement

To deploy NusaID to the cloud, we needed a robust container execution platform that supports:
1. Running containerized Go REST API services and Nginx-based React dashboards.
2. Autoscaling based on HTTP concurrency and request volume.
3. Blue-green / canary style revisions for zero-downtime deployments.
4. Instantaneous revision rollbacks (<60 seconds) in case of regression.
5. Codified infrastructure using OpenTofu and Terragrunt.

The standard industry default for container orchestration is often **Kubernetes (Azure Kubernetes Service / AKS)**. We evaluated whether full Kubernetes or a managed serverless container runtime like **Azure Container Apps (ACA)** represents the most appropriate engineering choice.

---

## 2. Decision Drivers

- **Core Engineering Principle**: *"Build the smallest real product that forces us to solve real production engineering problems. Never build features merely to add tech to a README."* (`AGENTS.md` & `PRD Section 41`).
- **Operational Burden**: A two-person or small API team should not spend hours managing Kubernetes control planes, node pools, CNI plugins, ingress controllers, or cluster version upgrades.
- **Rollback Speed & Simplicity**: The ability to shift traffic between immutable container revisions in seconds via simple metadata adjustments.
- **Cost Efficiency**: Avoiding flat cluster management fees and dedicated system node pools during low-traffic periods.

---

## 3. Considered Options

### Option 1: Azure Kubernetes Service (AKS)
- *Pros*: Complete ecosystem flexibility, standard Kubernetes manifests/helm charts, service mesh options (Istio/Linkerd), extensive community tooling.
- *Cons*:
  - Significant operational overhead (upgrades, node sizing, etcd backups, network policies).
  - High baseline cost ($70–$150+/month minimum for cluster node pools even at zero traffic).
  - Slow rollback workflows (often requiring pod rollout undos, image pulls, and readiness checks).
  - Violates Ponytail Rule 1 (over-engineering for an MVP API product).

### Option 2: Azure App Service (Web Apps for Containers)
- *Pros*: Simple configuration and managed TLS.
- *Cons*: Limited revision management; traffic shifting between deployment slots is slower and clunkier; higher minimum instance costs; autoscaling is less responsive than KEDA.

### Option 3: Azure Container Apps (ACA) (Selected)
- *Pros*:
  - Fully managed, serverless platform powered by Kubernetes, KEDA (Kubernetes Event-driven Autoscaling), and Envoy proxy under the hood, without exposing the Kubernetes management burden.
  - Native **Revision Management**: Each container deployment creates an immutable revision. Shifting traffic (e.g. 100% to previous revision) requires a single CLI or API call that executes in seconds without rebuilding or re-pulling images.
  - Scale-to-zero capability to optimize cost when development/staging environments are idle.
  - Native integration with Azure Key Vault via Managed Identity and Cloudflare edge proxy.
  - Clean OpenTofu module definition (`infra/modules/container_app/`).
- *Cons*: Less control over low-level kernel parameters or custom CNI configurations (not needed for NusaID).

---

## 4. Decision Outcome

**Chosen Option**: **Azure Container Apps (ACA)**

We deliberately chose Azure Container Apps over Kubernetes to demonstrate modern platform engineering judgment: choosing the simplest tool that satisfies all production requirements without operational self-harm.

### Key Capabilities Realized:
1. **Under-60-Second Rollbacks**:
   ```bash
   az containerapp revision set-traffic \
     --name ca-api-nusaid-prod \
     --resource-group rg-nusaid-prod \
     --revision-weight ca-api-nusaid-prod--1-4-0=100
   ```
   Traffic switches at the Envoy proxy level immediately. No pods are killed or rescheduled during rollback.
2. **KEDA Autoscaling**: Automatically scales replicas from 1 to 10 based on HTTP concurrent requests (e.g., scale out when concurrent requests exceed 50 per replica).
3. **Container Portability**: Because the application is packaged as a standard OCI Docker image (`Dockerfile`), the service can be migrated to AKS in the future with zero code changes if enterprise requirements ever demand it.

---

## 5. Consequences

### Positive
- **Zero Cluster Maintenance**: No node patching, zero control plane upgrades, and no Kubernetes security vulnerabilities to manage.
- **Fast Deployments**: Revision updates take less than 45 seconds in GitHub Actions.
- **Drastic Cost Reduction**: Costs scale directly with actual usage; staging can scale down to zero overnight.

### Negative / Trade-offs
- Azure Container Apps is an Azure-specific abstraction (though based on open standards like Envoy and KEDA). If migrating to AWS or GCP, the deployment target would map to AWS ECS / Fargate or Google Cloud Run.

---

## 6. References
- `AGENTS.md` - Rule 1 (Ponytail / Minimal Solution)
- `docs/deployment.md` - Environment Promotion Runbook
- `docs/rollback.md` - Emergency Rollback Runbook (<60s SLA)
- `infra/modules/container_app/` - OpenTofu Module
