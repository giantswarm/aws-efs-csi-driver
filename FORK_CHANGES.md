# Architecture: Upstream Dependency + GiantSwarm Extras

This document describes how the AWS EFS CSI Driver chart is structured.

## Approach

Instead of maintaining a fork of the upstream chart, we use the **unmodified upstream Helm chart as a dependency** (aliased as `upstream`) and keep only GiantSwarm-specific "extras" templates in our workload chart.

## Upstream Source

- **Upstream chart**: `aws-efs-csi-driver` from `https://kubernetes-sigs.github.io/aws-efs-csi-driver/`
- **Chart version**: 3.4.0 (appVersion 2.3.0)
- **Dependency alias**: `upstream` (values under `upstream:` key)

## Workload Chart Structure

```
helm/aws-efs-csi-driver/
  Chart.yaml            # upstream chart as dependency with alias "upstream"
  values.yaml           # upstream: section + extras at top level
  templates/
    _helpers.tpl         # GS labels helper (for extras resources only)
    network-policies.yaml # NetworkPolicy for CSI driver pods
    pss-exceptions.yaml   # Kyverno PolicyExceptions for Pod Security Standards
    vpa.yaml              # VerticalPodAutoscaler for controller and node
    storageclass-secret.yaml # Secrets for cross-account StorageClass support
```

## GiantSwarm Customizations

### Passed via upstream chart values (`upstream:` key)
| Customization | How |
|---------------|-----|
| Container registry (`gsoci.azurecr.io`) | `upstream.image.repository` (full path) |
| Sidecar registries | `upstream.sidecars.*.image.repository` (full path) |
| Controller scheduling (nodeSelector, tolerations, affinity) | `upstream.controller.*` |
| Node scheduling (tolerations, affinity) | `upstream.node.*` |
| Resources | `upstream.controller.resources`, `upstream.node.resources` |
| IRSA serviceAccount annotations | `upstream.controller.serviceAccount.annotations` (via bundle) |
| StorageClasses | `upstream.storageClasses[]` |
| GS labels on workloads | `upstream.controller.additionalLabels/podLabels`, `upstream.node.additionalLabels/podLabels` |

### GiantSwarm extras templates
| Template | Purpose |
|----------|---------|
| `network-policies.yaml` | NetworkPolicy allowing egress for CSI driver pods |
| `pss-exceptions.yaml` | Kyverno PolicyExceptions for privileged CSI containers |
| `vpa.yaml` | VPA for controller Deployment and node DaemonSet |
| `storageclass-secret.yaml` | Secrets for cross-account EFS StorageClass entries |

## Bundle Chart

The bundle chart (`helm/aws-efs-csi-driver-bundle/`) keeps flat values (backward compatible) and transforms them into the nested `upstream:` structure via the `giantswarm.workloadValues` helper in `_helpers.tpl`. Key transforms:
- Image `registry` + `repository` fields are combined into a single `repository` path
- GS labels are injected into `controller.additionalLabels/podLabels` and `node.additionalLabels/podLabels`
- IRSA annotation is computed from the crossplane-config ConfigMap

## Upgrade Process

To upgrade the upstream chart version:
1. Update `dependencies[0].version` in `helm/aws-efs-csi-driver/Chart.yaml`
2. Run `helm dependency update helm/aws-efs-csi-driver/`
3. Update image tags in `values.yaml` if needed
4. Test with `helm template` and deploy to a test cluster
