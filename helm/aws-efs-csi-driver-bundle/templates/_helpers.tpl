{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "aws-efs-csi-driver-bundle.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "aws-efs-csi-driver-bundle.fullname" -}}
{{- $name := .Chart.Name -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "aws-efs-csi-driver-bundle.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.AppVersion | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Resolve clusterID: use .Values.clusterID if set, otherwise derive from
the release name by stripping known chart name suffixes.
*/}}
{{- define "aws-efs-csi-driver-bundle.clusterID" -}}
{{- if .Values.clusterID -}}
  {{- .Values.clusterID -}}
{{- else -}}
  {{- $name := .Release.Name -}}
  {{- range $suffix := list (printf "-%s" $.Chart.Name) "-aws-efs-csi-driver-bundle" -}}
    {{- $name = trimSuffix $suffix $name -}}
  {{- end -}}
  {{- if eq $name .Release.Name -}}
    {{- fail "clusterID not set and cannot derive cluster name from release name" -}}
  {{- end -}}
  {{- $name -}}
{{- end -}}
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
giantswarm.io/cluster: {{ include "aws-efs-csi-driver-bundle.clusterID" . | quote }}
{{- end -}}

{{/*
AWS context for the IRSA role. Values win; anything unset is read from the
<clusterID>-crossplane-config ConfigMap in the release namespace, and a field
that resolves to nothing fails the render.
Returns YAML: include "aws-efs-csi-driver-bundle.aws" $ | fromYaml
*/}}
{{- define "aws-efs-csi-driver-bundle.aws" -}}
{{- $values := .Values.aws | default dict -}}
{{- $configMapName := printf "%s-crossplane-config" (include "aws-efs-csi-driver-bundle.clusterID" .) -}}
{{- $fallback := dict -}}
{{- if not (and $values.accountID $values.partition $values.oidcDomain) -}}
{{- $configMap := lookup "v1" "ConfigMap" .Release.Namespace $configMapName -}}
{{- if not (and $configMap $configMap.data $configMap.data.values) -}}
{{- fail (printf "ConfigMap %s/%s not found or has no values: set aws.accountID, aws.partition and aws.oidcDomain" .Release.Namespace $configMapName) -}}
{{- end -}}
{{- $fallback = fromYaml $configMap.data.values -}}
{{- end -}}
{{- $accountID := $values.accountID | default $fallback.accountID | default "" | toString -}}
{{- if not $accountID -}}
{{- fail (printf "aws.accountID is not set and ConfigMap %s/%s has no accountID" .Release.Namespace $configMapName) -}}
{{- end -}}
{{- $partition := $values.partition | default $fallback.awsPartition | default "" | toString -}}
{{- if not $partition -}}
{{- fail (printf "aws.partition is not set and ConfigMap %s/%s has no awsPartition" .Release.Namespace $configMapName) -}}
{{- end -}}
{{- $oidcDomains := $fallback.oidcDomains | default list | compact | uniq -}}
{{- if $values.oidcDomain -}}
{{- $oidcDomains = list $values.oidcDomain -}}
{{- end -}}
{{- if not $oidcDomains -}}
{{- fail (printf "aws.oidcDomain is not set and ConfigMap %s/%s has no oidcDomains" .Release.Namespace $configMapName) -}}
{{- end -}}
{{- dict "accountID" $accountID "partition" $partition "oidcDomains" $oidcDomains | toYaml -}}
{{- end -}}

{{/*
Get trust policy statements for all provided OIDC domains
*/}}
{{- define "aws-efs-csi-driver-bundle.trustPolicyStatements" -}}
{{- $aws := include "aws-efs-csi-driver-bundle.aws" . | fromYaml -}}
{{- range $index, $oidcDomain := $aws.oidcDomains -}}
{{- if not (eq $index 0) }}, {{ end }}{
  "Effect": "Allow",
  "Principal": {
    "Federated": "arn:{{ $aws.partition }}:iam::{{ $aws.accountID }}:oidc-provider/{{ $oidcDomain }}"
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
{{- $aws := include "aws-efs-csi-driver-bundle.aws" . | fromYaml -}}
{{- $clusterID := (include "aws-efs-csi-driver-bundle.clusterID" .) -}}
{{- $_ := set .Values.controller.serviceAccount.annotations "eks.amazonaws.com/role-arn" (printf "arn:%s:iam::%s:role/%s-aws-efs-csi-driver-role" $aws.partition $aws.accountID $clusterID) -}}
{{- if and (not .Values.clusterName) -}}
{{- $_ := set .Values "clusterName" $clusterID -}}
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
{{- $bundleOnlyKeys := list "ociRepositoryUrl" "clusterID" "clusterName" "aws" -}}
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

{{/* Preserve the original chart name so selectors stay compatible with pre-dependency upgrades */}}
{{- $_ := set $upstreamValues "nameOverride" "aws-efs-csi-driver" -}}

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
