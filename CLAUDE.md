# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Giant Swarm managed app for AWS EFS CSI Driver. This uses the unmodified upstream kubernetes-sigs/aws-efs-csi-driver Helm chart as a dependency, with GiantSwarm-specific extras templates.

## Architecture

**Two-chart pattern:**
- `helm/aws-efs-csi-driver-bundle/` - Installed on the **management cluster**. Creates:
  - Crossplane IAM Role for IRSA (EFS permissions)
  - Flux OCIRepository + HelmRelease to deploy the workload chart
  - ConfigMap with values for the workload chart (transforms flat values into nested `upstream:` structure)
- `helm/aws-efs-csi-driver/` - Deployed to **workload clusters** via Flux. Contains:
  - Upstream chart as a Helm dependency (alias: `upstream`)
  - GiantSwarm extras templates (NetworkPolicy, PSS exceptions, VPA, StorageClass secrets)

**Upstream as dependency (not fork):**
- The upstream chart is declared as a dependency in `Chart.yaml` with alias `upstream`
- Values for the upstream chart go under the `upstream:` key in `values.yaml`
- GiantSwarm extras templates live in `templates/` and use top-level values
- See [FORK_CHANGES.md](FORK_CHANGES.md) for full architecture details

## Key Values

The bundle's `giantswarm.workloadValues` helper transforms flat bundle values into the nested structure the workload chart expects:
- Image `registry` + `repository` → single `repository` path for upstream chart
- GS labels injected into `upstream.controller.additionalLabels/podLabels` and `upstream.node.additionalLabels/podLabels`
- IRSA annotation computed from crossplane-config ConfigMap

Important values:
- `clusterID` - Required (bundle). Used for IRSA role naming and resource prefixes
- `upstream.controller.serviceAccount.annotations` - Set automatically by the bundle for IRSA
- `global.podSecurityStandards.enforced` - Controls PSS exception creation
- `networkPolicy.enabled` - Controls NetworkPolicy creation

## CI/CD

CircleCI packages and pushes both charts to `giantswarm-catalog` on tags matching `v*.*.*`. Uses the `architect` orb.

## Upstream Upgrade Process

1. Update `dependencies[0].version` in `helm/aws-efs-csi-driver/Chart.yaml`
2. Run `helm dependency update helm/aws-efs-csi-driver/`
3. Update image tags in `values.yaml` if needed
4. Test with `helm template` and deploy to a test cluster
