{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "aws-efs-csi-driver-bundle.name" -}}
{{- default .Chart.Name .Values.bundleNameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "aws-efs-csi-driver-bundle.fullname" -}}
{{- if .Values.fullBundleNameOverride -}}
{{- .Values.fullBundleNameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.bundleNameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "aws-efs-csi-driver-bundle.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "aws-efs-csi-driver-bundle.labels" -}}
app.kubernetes.io/name: {{ include "aws-efs-csi-driver-bundle.name" . }}
helm.sh/chart: {{ include "aws-efs-csi-driver-bundle.chart" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
giantswarm.io/service-type: "managed"
application.giantswarm.io/team: {{ index .Chart.Annotations "application.giantswarm.io/team" | quote }}
giantswarm.io/cluster: {{ .Values.clusterID | quote }}
{{- end -}}

{{/*
Fetch crossplane config ConfigMap data
*/}}
{{- define "aws-efs-csi-driver-bundle.crossplaneConfigData" -}}
{{- $clusterName := .Values.clusterID -}}
{{- $configmap := (lookup "v1" "ConfigMap" .Release.Namespace (printf "%s-crossplane-config" $clusterName)) -}}
{{- $cmvalues := dict -}}
{{- if and $configmap $configmap.data $configmap.data.values -}}
  {{- $cmvalues = fromYaml $configmap.data.values -}}
{{- else -}}
  {{- fail (printf "Crossplane config ConfigMap %s-crossplane-config not found in namespace %s or has no data" $clusterName .Release.Namespace) -}}
{{- end -}}
{{- $cmvalues | toYaml -}}
{{- end -}}

{{/*
Get trust policy statements for all provided OIDC domains
*/}}
{{- define "aws-efs-csi-driver-bundle.trustPolicyStatements" -}}
{{- $cmvalues := (include "aws-efs-csi-driver-bundle.crossplaneConfigData" .) | fromYaml -}}
{{- range $index, $oidcDomain := $cmvalues.oidcDomains -}}
{{- if not (eq $index 0) }}, {{ end }}{
  "Effect": "Allow",
  "Principal": {
    "Federated": "arn:{{ $cmvalues.awsPartition }}:iam::{{ $cmvalues.accountID }}:oidc-provider/{{ $oidcDomain }}"
  },
  "Action": "sts:AssumeRoleWithWebIdentity",
  "Condition": {
    "StringLike": {
      "{{ $oidcDomain }}:sub": "system:serviceaccount:kube-system:{{ $.Values.controller.serviceAccount.name }}"
    }
  }
}
{{- end -}}
{{- end -}}

{{/*
Set Giant Swarm specific values — computes IRSA role ARN.
*/}}
{{- define "giantswarm.setValues" -}}
{{- $cmvalues := (include "aws-efs-csi-driver-bundle.crossplaneConfigData" .) | fromYaml -}}
{{- $_ := set .Values.controller.serviceAccount.annotations "eks.amazonaws.com/role-arn" (printf "arn:%s:iam::%s:role/%s-aws-efs-csi-driver-role" $cmvalues.awsPartition $cmvalues.accountID .Values.clusterID) -}}
{{- if and (not .Values.clusterName) -}}
{{- $_ := set .Values "clusterName" .Values.clusterID -}}
{{- end -}}
{{- end -}}

{{/*
Reusable: combine GS split registry+repository into upstream single repository.
Preserves all other keys (tag, pullPolicy, etc.) from the input dict.
*/}}
{{- define "giantswarm.combineImage" -}}
{{- $result := deepCopy . -}}
{{- $_ := set $result "repository" (printf "%s/%s" .registry .repository) -}}
{{- $_ := unset $result "registry" -}}
{{- $result | toYaml -}}
{{- end -}}

{{/*
Transform flat bundle values into the nested workload chart structure.
The workload chart expects:
  - upstream: {} — values for the upstream subchart dependency
  - networkPolicy: {} — extras
  - verticalPodAutoscaler: {} — extras
  - global: {} — extras
  - storageClasses: [] — extras (for cross-account secrets)

Keys listed in $bundleOnlyKeys and $extrasKeys are excluded from upstream.
Any other key in .Values passes through to upstream automatically.
*/}}
{{- define "giantswarm.workloadValues" -}}
{{- include "giantswarm.setValues" . -}}
{{- $upstreamValues := dict -}}

{{/* Keys that belong to the bundle chart itself (never forwarded) */}}
{{- $bundleOnlyKeys := list "nameOverride" "fullnameOverride" "bundleNameOverride" "fullBundleNameOverride" "ociRepositoryUrl" "clusterID" "clusterName" -}}
{{/* Keys forwarded as workload extras (not under upstream:) */}}
{{- $extrasKeys := list "networkPolicy" "verticalPodAutoscaler" "global" -}}
{{/* Keys with special handling */}}
{{- $specialKeys := list "image" "sidecars" "controller" "node" "storageClasses" -}}
{{- $reservedKeys := concat $bundleOnlyKeys $extrasKeys $specialKeys -}}

{{/* Image: combine GS split format */}}
{{- $_ := set $upstreamValues "image" (include "giantswarm.combineImage" .Values.image | fromYaml) -}}

{{/* Sidecars: combine GS split format for each */}}
{{- $sidecars := deepCopy .Values.sidecars -}}
{{- range $name, $sidecar := .Values.sidecars -}}
  {{- if and $sidecar.image $sidecar.image.registry $sidecar.image.repository -}}
    {{- $_ := set (index $sidecars $name) "image" (include "giantswarm.combineImage" $sidecar.image | fromYaml) -}}
  {{- end -}}
{{- end -}}
{{- $_ := set $upstreamValues "sidecars" $sidecars -}}

{{/* Controller + Node: direct pass-through (GS labels already in values) */}}
{{- $_ := set $upstreamValues "controller" (deepCopy .Values.controller) -}}
{{- $_ := set $upstreamValues "node" (deepCopy .Values.node) -}}

{{/* storageClasses: forwarded to both upstream and workload extras */}}
{{- if .Values.storageClasses -}}
{{- $_ := set $upstreamValues "storageClasses" .Values.storageClasses -}}
{{- end -}}

{{/* Pass through any non-reserved value to upstream (e.g. useFIPS, imagePullSecrets) */}}
{{- range $key, $val := .Values -}}
  {{- if not (has $key $reservedKeys) -}}
  {{- $_ := set $upstreamValues $key $val -}}
  {{- end -}}
{{- end -}}

{{/* Assemble workload values: upstream + extras */}}
{{- $workloadValues := dict "upstream" $upstreamValues -}}
{{- $_ := set $workloadValues "networkPolicy" .Values.networkPolicy -}}
{{- $_ := set $workloadValues "verticalPodAutoscaler" .Values.verticalPodAutoscaler -}}
{{- $_ := set $workloadValues "global" .Values.global -}}
{{- if .Values.storageClasses -}}
{{- $_ := set $workloadValues "storageClasses" .Values.storageClasses -}}
{{- end -}}

{{- $workloadValues | toYaml -}}
{{- end -}}
