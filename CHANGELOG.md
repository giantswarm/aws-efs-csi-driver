# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Merge both the bundle and app chart into the same repository. Now the AWS EFS Driver app and the bundle containing the necessary IAM resources (managed by Crossplane) will live in this repository.

## [2.1.5] - 2025-10-07

### Fixed

- Use the container registry helm value when templating the daemonset.

## [2.1.4] - 2025-09-19

### Changed

- Remove resource limits, causing issues with VPA on boot.

### Fix

- Add PSS for non-root

### Added

- Add CI helm values file to check Helm rendering during CI.

## [2.1.3] - 2025-09-17

### Changed

- Chart: Fix templating.

## [2.1.2] - 2025-09-11

### Changed

- Increase `minAllowed` memory in VPA.

## [2.1.1] - 2025-09-11

### Fixed

- Changed `node` controller `nodeAffinity` to only schedule into the worker nodes, to keep the same behavior as we had before removing the `node-role.kubernetes.io/worker` label.

## [2.1.0] - 2025-09-10

### Changed

- Removed default `nodeSelector` that were relying on the `node-role.kubernetes.io/worker` label, as that's deprecated and will be removed.
- Add VPA.
- Add standard labels to all manifests.
- Move PSS to policy-exceptions namespace.

## [2.0.0] - 2025-08-28

⚠️ Attention: Major release [2.0.0] contains breaking changes. You need to uninstall the previous app before upgrading to the new release. Your EFS files and mounts will not be deleted, your pods should continue running as normal with no downtime. ⚠️

### Changed

- Upgrade EFS driver
- Update Kyverno PolicyException apiVersion to v2.

## [1.3.0] - 2024-02-08

### Fixed

- Add even more PSS exceptions to make `node` component run on 1.25.

## [1.2.3] - 2024-01-16

### Fixed

- Fix PSS to allow using hostNetwork in agents.

## [1.2.2] - 2023-12-21

# Changed
- Use hostNetwork in agents in order to be able to recover from reboots. EFS tunnels are not stuck anymore.

## [1.2.1] - 2023-12-20

### Changed

- Configure `gsoci.azurecr.io` as the default container image registry.

## [1.2.0] - 2023-11-22

⚠️ Attention: Minor release [1.2.0] contains breaking changes in default user values! The name of the service account changed to `efs-csi-sa` for both components. This affects clusters with already configured IRSA roles. You either need to update the trust identity policy on the role to match the new service account or set service account names to the original values via the values file. ⚠️

### Updated

- Extract registry from container images to allow set it for all images on the value level.
- Change node-selector values to fit the new scheme.
- Configure the same service account for both controller and node to simplify IRSA role permissions.

## [1.1.0] - 2023-10-30

### Changed

- Force-disable PSP-related resources when `global.podSecurityStandards.enforced` value is true.

### Added

- Add PSS exceptions.

## [1.0.0] - 2023-09-27

### Changed

- Breaking Change: Move chart to "giantswarm" catalog from "giantswarm-playground"
- Disabled VPA
- Bumped image to v1.6.0

## [0.8.1] - 2023-06-23

### Fixed

- Bump app version in chart.yaml too

## [0.8.0] - 2023-06-23

### Changed

- Bumped image to v1.5.7

## [0.7.2] - 2023-06-09

### Fixed

- Added `projected` volume type to csi-node PSP to allow the user of IRSA.

## [0.7.1] - 2022-11-15

**WARNING** Please ensure you're running `kiam-app` with App Version `v2.5.1` or higher.

### Changed

- Update aws-efs-csi-driver version to `v1.4.5`.

## [0.7.0] - 2022-11-15

**WARNING** Please ensure you're running `kiam-app` with App Version `v2.5.1` or higher.

This is necessary to allow efs-csi-driver to get the `Identity Document` from `Instance Metadata Service`.
You can verify this by describing `kiam-agent` DaemonSet and checking if `--allow-route-regexp="/latest/*"` is set.

### Changed

- Update aws-efs-csi-driver version to `v1.4.4`.
- Disable `hostNetwork`.

## [0.6.4] - 2022-07-26

- Defaulting deployment strategy to `Recreate` for the controller

### Fixed

## [0.6.3] - 2022-07-26

### Fixed

- Added the possibility to override the deployment strategy for the controller

## [0.6.2] - 2022-07-25

### Fixed

- Added `projected` to the list of permitted volumes in the `psp-csi-controller`.

## [0.6.1] - 2022-07-19

### Fixed

- Respect the `controller.serviceAccount.create` values flag

### Added

- values.schema.json

## [0.6.0] - 2022-06-15

### Changed

- Remove `imagePullSecrets` from values.yaml

## [0.5.0] - 2022-05-25

### Changed

- Use different ports to avoid collision with ebs controller using ports 9909 and 9809
- Updated chart to upstream 2.2.6
- Improve tolerations to deploy in all worker nodes
- Remove root security context from pods

## [0.4.0] - 2022-04-28

### Changed

- Update efs-csi-driver to v1.3.8.
- Adjust node selector and tolerations.

## [0.3.0] - 2022-03-22

### Added

- Add VerticalPodAutoscaler CR.

## [0.2.0] - 2021-08-27

### Changed

- Update efs-csi-driver to v1.3.3.

## [0.1.2] - 2021-05-06

### Fixed

- Include optional storage classes and tags.

## [0.1.1] - 2021-05-05

### Changed

- Update efs-csi-driver to v1.2.1.

## [0.1.0] - 2021-04-06

### Changed

- Update efs-csi-driver to v1.2.0.

## [0.0.7] - 2021-02-01

## [0.0.6] - 2021-01-29

## [0.0.5] - 2021-01-27

## [0.0.4] - 2021-01-27

## [0.0.3] - 2021-01-27

## [0.0.2] - 2021-01-27

## [0.0.1] - 2021-01-27

[Unreleased]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v2.1.5...HEAD
[2.1.5]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v2.1.4...v2.1.5
[2.1.4]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v2.1.3...v2.1.4
[2.1.3]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v2.1.2...v2.1.3
[2.1.2]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v2.1.1...v2.1.2
[2.1.1]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v2.0.0...v2.0.0
[2.0.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v1.2.3...v1.3.0
[1.2.3]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v1.2.2...v1.2.3
[1.2.2]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.8.1...v1.0.0
[0.8.1]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.7.2...v0.8.0
[0.7.2]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.6.4...v0.7.0
[0.6.4]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.6.3...v0.6.4
[0.6.3]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.6.2...v0.6.3
[0.6.2]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.0.7...v0.1.0
[0.0.7]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.0.6...v0.0.7
[0.0.6]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.0.5...v0.0.6
[0.0.5]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.0.4...v0.0.5
[0.0.4]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.0.3...v0.0.4
[0.0.3]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/giantswarm/aws-efs-csi-driver/releases/tag/v0.0.1
