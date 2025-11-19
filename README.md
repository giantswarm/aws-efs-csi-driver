[![CircleCI](https://circleci.com/gh/giantswarm/aws-efs-csi-driver.svg?style=shield)](https://circleci.com/gh/giantswarm/aws-efs-csi-driver)

# aws-efs-csi-driver chart

Giant Swarm offers a `aws-efs-csi-driver-bundle` Managed App which can be installed in tenant clusters.
Here we define the `aws-efs-csi-driver-bundle`, `aws-efs-csi-driver` charts with their templates and default configuration.

## Architecture

This repository contains two Helm charts:

- `helm/aws-efs-csi-driver-bundle/`: Main chart installed on the management cluster, contains the workload cluster chart and the required AWS IAM role.
- `helm/aws-efs-csi-driver/`: Workload cluster chart that contains the actual EFS driver setup.

Users only need to install the bundle chart on the management cluster, which in turn will deploy the workload cluster chart.

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

## Credit

* https://github.com/kubernetes-sigs/aws-efs-csi-driver
