[![CircleCI](https://circleci.com/gh/giantswarm/aws-efs-csi-driver.svg?style=shield)](https://circleci.com/gh/giantswarm/aws-efs-csi-driver)

# aws-efs-csi-driver chart

Giant Swarm offers a `aws-efs-csi-driver-bundle` Managed App which can be installed in tenant clusters.
Here we define the `aws-efs-csi-driver-bundle`, `aws-efs-csi-driver` charts with their templates and default configuration.

## Key terminology

This repository uses three terms to describe where configuration lives and how it flows:

| Term | Where it lives | What it does |
|------|---------------|--------------|
| **Bundle** | Management cluster (`helm/aws-efs-csi-driver-bundle/`) | Values consumed only by the bundle chart. They drive management-cluster resources (Crossplane IAM role, Flux resources) and are **never** forwarded to the workload cluster. Examples: `clusterID`, `ociRepositoryUrl`. |
| **Upstream** | Workload cluster, under the `upstream:` key | Values routed to the **unmodified** [upstream Helm chart](https://github.com/kubernetes-sigs/aws-efs-csi-driver) (declared as a subchart dependency with alias `upstream`). These control the CSI driver itself: images, controller/node settings, service accounts, tolerations, etc. The bundle helper transforms flat values (e.g. split `registry`+`repository`) into the nested `upstream:` structure the subchart expects. |
| **Extras** | Workload cluster, at the top level (not under `upstream:`) | Values consumed by Giant Swarm-specific templates that live alongside the upstream subchart. These add operational features the upstream chart doesn't provide: `networkPolicy`, `verticalPodAutoscaler`, `global.podSecurityStandards`, and `storageClasses`. |

Both the bundle's `values.yaml` and the workload chart's `values.yaml` are annotated with section headers (`BUNDLE-ONLY`, `UPSTREAM`, `EXTRAS`) so you can see at a glance where each value ends up.

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

**Locally (against an existing cluster):**

The apptest-framework requires `E2E_KUBECONFIG_CONTEXT` to match a known provider name (`capa`, `eks`, `capv`, `capz`, etc.), so you need a kubeconfig where the MC context is named accordingly.

1. Create a kubeconfig with the correct context name:

```bash
# Login to the MC with a context named "capa"
KUBECONFIG=/tmp/e2e-kubeconfig.yaml tsh kube login <mc-name> --set-context-name=capa
```

2. Set environment variables:

```bash
export E2E_KUBECONFIG="/tmp/e2e-kubeconfig.yaml"
export E2E_KUBECONFIG_CONTEXT="capa"
export E2E_APP_VERSION="<chart-version-from-test-catalog>"

# To reuse an existing workload cluster (skip cluster creation):
export E2E_WC_NAME="<cluster-name>"
export E2E_WC_NAMESPACE="org-<org-name>"
export E2E_WC_KEEP="true"  # Prevent cluster deletion after tests
```

3. Compile and run the test binary (the framework finds `config.yaml` relative to the binary, so `go test` alone won't work):

```bash
cd tests/e2e
go test -c -o suites/basic/basic.test ./suites/basic/
cd suites/basic
./basic.test -test.v -test.timeout 60m
```

**Finding the chart version:**

For branch builds, check the test catalog for the version corresponding to your commit:

```bash
curl -s "https://giantswarm.github.io/giantswarm-test-catalog/index.yaml" | grep -A5 "aws-efs-csi-driver-bundle:"
```

For tagged releases, use the version from `giantswarm` catalog (e.g. `3.2.0`).

## Credit

* https://github.com/kubernetes-sigs/aws-efs-csi-driver
