# GiantSwarm Fork Changes

This document tracks all customizations made to the upstream AWS EFS CSI Driver chart.

## Upstream Source

- **Upstream**: https://github.com/kubernetes-sigs/aws-efs-csi-driver
- **GiantSwarm Fork**: https://github.com/giantswarm/aws-efs-csi-driver-upstream
- **Last synced**: 2026-02-03 (upstream v2.3.0)

## Files Added by GiantSwarm

| File | Purpose |
|------|---------|
| `templates/network-policies.yaml` | Network policies for pod communication |
| `templates/pss-exceptions.yaml` | Kyverno PolicyExceptions for Pod Security Standards |
| `templates/vpa.yaml` | VerticalPodAutoscaler resources for controller and node |
| `templates/storageclass-secret.yaml` | Secret for cross-account StorageClass support |

## Files Modified from Upstream

| File | Changes |
|------|---------|
| `templates/_helpers.tpl` | Added common GiantSwarm labels helper |
| `templates/controller-deployment.yaml` | Added container registry templating, GiantSwarm labels |
| `templates/controller-pdb.yaml` | Added GiantSwarm labels |
| `templates/controller-serviceaccount.yaml` | Added IRSA role annotation using `clusterID` value |
| `templates/csidriver.yaml` | Added GiantSwarm labels |
| `templates/node-daemonset.yaml` | Added container registry templating, GiantSwarm labels |
| `templates/node-serviceaccount.yaml` | Added GiantSwarm labels |
| `templates/storageclass.yaml` | Added cross-account support with secrets |

## values.yaml Customizations

The following values differ from upstream defaults:

### Image Configuration
- Uses `gsoci.azurecr.io` registry with `giantswarm/*` image paths
- Separate `registry` field for all images

### Controller Defaults
- `nodeSelector`: Schedules on control-plane nodes
- `tolerations`: Includes control-plane toleration
- `affinity`: Pod anti-affinity for HA
- `resources.requests`: Set to 100m CPU, 128Mi memory
- `serviceAccount.name`: `efs-csi-sa` (shared with node for IRSA simplicity)

### Node Defaults
- `affinity`: Excludes control-plane nodes
- `resources.requests`: Set to 100m CPU, 128Mi memory

### GiantSwarm-specific Values
- `verticalPodAutoscaler`: VPA configuration for both components
- `global.podSecurityStandards.enforced`: PSS enforcement flag
- `clusterID`: Used for IRSA role naming

## Sync Process

To sync with upstream:
1. Run `/upstream-sync` to merge upstream changes into the GiantSwarm fork
2. Run `vendir sync` to pull templates
3. Update `values.yaml` manually (not managed by vendir)
4. Regenerate `values.schema.json` with `helm schema-gen`
5. Update CHANGELOG.md
