[![CircleCI](https://circleci.com/gh/giantswarm/aws-efs-csi-driver.svg?style=shield)](https://circleci.com/gh/giantswarm/aws-efs-csi-driver)

# aws-efs-csi-driver chart

Giant Swarm offers a `aws-efs-csi-driver-bundle` Managed App which can be installed in tenant clusters.
Here we define the `aws-efs-csi-driver-bundle`, `aws-efs-csi-driver` charts with their templates and default configuration.

## Architecture

This repository uses a **two-chart bundle pattern** to deploy the AWS EFS CSI driver across Giant Swarm's management and workload clusters.

### Overview

```
 Management Cluster                          Workload Cluster
 ──────────────────────                      ──────────────────────

 ┌─────────────────────────────────┐
 │  aws-efs-csi-driver-bundle      │
 │  (installed via App CR)         │
 │                                 │
 │  Bundle values.yaml             │
 │  ┌───────────────────────────┐  │
 │  │ image:                    │  │
 │  │   registry: gsoci...      │──┼──┐
 │  │   repository: giantswarm/ │  │  │  combined into single
 │  │ sidecars: ...             │  │  │  upstream "repository" field
 │  │ controller:               │  │  │
 │  │   labels, resources,      │──┼──┤  passed through as
 │  │   tolerations, affinity   │  │  │  upstream.controller
 │  │ node:                     │──┼──┤  upstream.node
 │  │   labels, resources,      │  │  │
 │  │   affinity                │  │  │
 │  │ networkPolicy, VPA,       │──┼──┤  forwarded as workload
 │  │   global                  │  │  │  extras (not under upstream:)
 │  └───────────────────────────┘  │  │
 │                                 │  │
 │  _helpers.tpl                   │  │
 │  ┌───────────────────────────┐  │  │
 │  │ giantswarm.workloadValues │  │  │
 │  │ • Combines registry+repo  │◄─┼──┘
 │  │ • Computes IRSA role ARN  │  │
 │  │ • Builds nested structure │  │
 │  └──────────┬────────────────┘  │
 │             │                   │
 │             ▼                   │
 │  ┌───────────────────────────┐  │        ┌──────────────────────────┐
 │  │ ConfigMap                 │  │        │ aws-efs-csi-driver       │
 │  │ {clusterID}-aws-efs-csi- │  │        │ (deployed via Flux)      │
 │  │ driver-config             │  │        │                          │
 │  │                           │  │        │ values.yaml (extras)     │
 │  │  data.values:             │  │        │ ┌──────────────────────┐ │
 │  │   upstream:               │──┼────┐   │ │ networkPolicy:       │ │
 │  │     image: ...            │  │    │   │ │   enabled: true      │ │
 │  │     controller: ...       │  │    │   │ │ verticalPodAuto...   │ │
 │  │     node: ...             │  │    │   │ │ global: ...          │ │
 │  │   networkPolicy: ...      │  │    │   │ │ storageClasses: []   │ │
 │  │   verticalPodAuto...: ... │  │    │   │ └──────────────────────┘ │
 │  │   global: ...             │  │    │   │                          │
 │  └───────────────────────────┘  │    │   │ Upstream subchart        │
 │                                 │    │   │ (alias: upstream)        │
 │  ┌───────────────────────────┐  │    │   │ ┌──────────────────────┐ │
 │  │ Crossplane IAM Role       │  │    │   │ │ aws-efs-csi-driver   │ │
 │  │ {clusterID}-aws-efs-csi- │  │    │   │ │ v3.4.0               │ │
 │  │ driver-role               │  │    │   │ │                      │ │
 │  │ • IRSA trust policy       │  │    └──►│ │ Reads values from    │ │
 │  │ • EFS IAM permissions     │  │        │ │ upstream: key via    │ │
 │  └───────────────────────────┘  │        │ │ Helm deep-merge      │ │
 │                                 │        │ └──────────────────────┘ │
 │  ┌───────────────────────────┐  │        │                          │
 │  │ OCIRepository + HelmRelease│  │        │ GS extras templates     │
 │  │ (Flux resources)          │──┼───────►│ ┌──────────────────────┐ │
 │  │ • Points to workload chart│  │        │ │ NetworkPolicy        │ │
 │  │ • valuesFrom: ConfigMap   │  │        │ │ VPA (controller+node)│ │
 │  └───────────────────────────┘  │        │ │ PSS exceptions       │ │
 │                                 │        │ │ StorageClass secrets  │ │
 └─────────────────────────────────┘        │ └──────────────────────┘ │
                                            └──────────────────────────┘
```

### Charts

| Chart | Cluster | Purpose |
|-------|---------|---------|
| `helm/aws-efs-csi-driver-bundle/` | Management | Orchestrator. Creates the IAM role (Crossplane), Flux resources, and a ConfigMap with computed values for the workload chart. |
| `helm/aws-efs-csi-driver/` | Workload | Driver. Wraps the unmodified [upstream chart](https://github.com/kubernetes-sigs/aws-efs-csi-driver) as a dependency (alias `upstream`) and adds GS extras (NetworkPolicy, VPA, PSS exceptions, StorageClass secrets). |

### Value flow

The bundle chart's `_helpers.tpl` contains a `giantswarm.workloadValues` helper that transforms flat bundle values into the nested structure the workload chart expects:

1. **Image transformation** -- The bundle stores images in GS split format (`registry` + `repository`). The helper combines them into a single `repository` field that the upstream chart expects (e.g. `gsoci.azurecr.io` + `giantswarm/aws-efs-csi-driver` becomes `gsoci.azurecr.io/giantswarm/aws-efs-csi-driver`).

2. **IRSA annotation** -- The helper looks up the cluster's `crossplane-config` ConfigMap to compute the IAM role ARN and injects it into `controller.serviceAccount.annotations`.

3. **Upstream vs. extras routing** -- Values are split into two groups:
   - **Upstream values** (`image`, `sidecars`, `controller`, `node`, `storageClasses`, and any non-reserved key) are nested under `upstream:` so Helm routes them to the subchart.
   - **Extras values** (`networkPolicy`, `verticalPodAutoscaler`, `global`) stay at the top level for the GS extras templates.

4. **Bundle-only keys** (`clusterID`, `ociRepositoryUrl`, name overrides) are excluded from the workload chart entirely.

The resulting structure is written to a ConfigMap, which Flux's HelmRelease reads via `valuesFrom`. At deploy time, Helm deep-merges these values with the workload chart's own `values.yaml` defaults.

### Why this pattern

- **Unmodified upstream** -- The upstream chart is a direct Helm dependency, not a fork. Upgrades are a version bump in `Chart.yaml` + `helm dependency update`.
- **Separation of concerns** -- IAM and Flux orchestration live on the management cluster. The workload chart knows nothing about Crossplane or multi-cluster setup.
- **Single App CR** -- Users install the bundle once on the management cluster. Everything else is automated via Flux.

## Installation

Install the chart on the management cluster using an App CR:

```yaml
apiVersion: application.giantswarm.io/v1alpha1
kind: App
metadata:
  name: coyote-aws-efs-csi-driver-bundle
  namespace: org-acme
spec:
  catalog: giantswarm
  config:
    configMap:
      name: coyote-cluster-values
      namespace: org-acme
  kubeConfig:
    inCluster: true
  name: aws-efs-csi-driver-bundle
  namespace: org-acme
  version: 3.0.0
```

Note: remember that when mounting an EFS volume using the CSI driver, the Security Group attached to the EFS file system must allow inbound traffic to port 2049 from the cluster nodes Security Group, which has a name like `<cluster-id>-node` (following up on the example above: `coyote-node`).

## Upgrade from v2.x.x to v3.x.x

v3.x.x introduces a breaking change: a new installation method for the app. Please review the [v3 release notes](https://github.com/giantswarm/aws-efs-csi-driver/releases/tag/v3.0.0) for detailed upgrade instructions and migration steps.

## Testing

### E2E tests

End-to-end tests live in `tests/e2e/` and use the [apptest-framework](https://github.com/giantswarm/apptest-framework). They install the bundle on a CAPA management cluster, provision real EFS infrastructure via Crossplane, and validate dynamic provisioning with access points on a workload cluster.

**What the tests cover:**

1. Crossplane creates an EFS filesystem, security group, and mount targets in the workload cluster's VPC.
2. The bundle's HelmRelease reaches Ready on the MC.
3. The `efs-csi-controller` Deployment and `efs-csi-node` DaemonSet are running on the WC.
4. A StorageClass with `provisioningMode: efs-ap` dynamically provisions an access point.
5. A writer Pod writes data to the volume; a reader Pod reads it back (verifying RWX shared access).
6. All EFS infrastructure (access points, mount targets, filesystem, security group) is cleaned up.

**From CI:**

```
/run app-test-suites
```

**Locally:**

```bash
export E2E_KUBECONFIG=/path/to/mc-kubeconfig
export E2E_KUBECONFIG_CONTEXT=your-mc-context
export E2E_APP_VERSION=latest

cd tests/e2e
go test ./suites/basic -v -timeout 60m
```

## Credit

* https://github.com/kubernetes-sigs/aws-efs-csi-driver
