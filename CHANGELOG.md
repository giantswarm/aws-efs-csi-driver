# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/giantswarm/aws-efs-csi-driver/compare/v0.5.0...HEAD
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
