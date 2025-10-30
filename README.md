[![CircleCI](https://circleci.com/gh/giantswarm/aws-efs-csi-driver.svg?style=shield)](https://circleci.com/gh/giantswarm/aws-efs-csi-driver)

# aws-efs-csi-driver chart

Giant Swarm offers a aws-efs-csi-driver Managed App which can be installed in tenant clusters.
Here we define the aws-efs-csi-driver chart with its templates and default configuration.

## Architecture

This repository contains two Helm charts:

- `helm/aws-efs-csi-driver-app/`: Main chart installed on the management cluster, contains the workload cluster chart and the required AWS IAM role.
- `helm/aws-efs-csi-driver/`: Workload cluster chart that contains the actual EFS driver setup.

Users only need to install the main App chart on the management cluster, which in turn will deploy the workload cluster chart.

## Installation

Install the chart on the management cluster using an App CR:

```yaml
apiVersion: application.giantswarm.io/v1alpha1
kind: App
metadata:
  name: coyote-aws-efs-csi-driver-app
  namespace: org-acme
spec:
  catalog: giantswarm
  config:
    configMap:
      name: coyote-cluster-values
      namespace: org-acme
  kubeConfig:
    inCluster: true
  name: aws-efs-csi-driver-app
  namespace: org-acme
  version: 2.1.5
```

## Credit

* https://github.com/kubernetes-sigs/aws-efs-csi-driver
