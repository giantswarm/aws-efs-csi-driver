# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Giant Swarm managed app for AWS EFS CSI Driver. This wraps the upstream kubernetes-sigs/aws-efs-csi-driver Helm chart with Giant Swarm customizations.

## Architecture

**Two-chart pattern:**
- `helm/aws-efs-csi-driver-bundle/` - Installed on the **management cluster**. Creates:
  - Crossplane IAM Role for IRSA (EFS permissions)
  - Flux OCIRepository + HelmRelease to deploy the workload chart
  - ConfigMap with values for the workload chart
- `helm/aws-efs-csi-driver/` - Deployed to **workload clusters** via Flux. Contains the actual CSI driver resources.

**Template sync via vendir:**
- Templates in `helm/aws-efs-csi-driver/templates/` are synced from [giantswarm/aws-efs-csi-driver-upstream](https://github.com/giantswarm/aws-efs-csi-driver-upstream)
- The upstream fork contains Giant Swarm modifications tracked in [FORK_CHANGES.md](FORK_CHANGES.md)
- `values.yaml` and `values.schema.json` are NOT synced - they're maintained locally
- Changes can't be dont in the templates folder directly, we need to change it in the fork repository and sync via vendir

## Key Values

The bundle's `giantswarm.setValues` helper ([_helpers.tpl:89-96](helm/aws-efs-csi-driver-bundle/templates/_helpers.tpl#L89-L96)) dynamically sets the IRSA role ARN annotation from the cluster's crossplane-config ConfigMap.

Important values:
- `clusterID` - Required. Used for IRSA role naming and resource prefixes
- `controller.serviceAccount.annotations` - Set automatically by the bundle for IRSA
- `global.podSecurityStandards.enforced` - Controls PSS exception creation

## CI/CD

CircleCI packages and pushes both charts to `giantswarm-catalog` on tags matching `v*.*.*`. Uses the `architect` orb.

## Upstream Sync Process

Use `/gs-base:upgrade-vendir-app` command
