{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "aws-efs-csi-driver-app.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "aws-efs-csi-driver-app.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
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
{{- define "aws-efs-csi-driver-app.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "aws-efs-csi-driver-app.labels" -}}
app.kubernetes.io/name: {{ include "aws-efs-csi-driver-app.name" . }}
helm.sh/chart: {{ include "aws-efs-csi-driver-app.chart" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
giantswarm.io/service-type: "managed"
application.giantswarm.io/team: {{ index .Chart.Annotations "application.giantswarm.io/team" | quote }}
{{- end -}}

{{/*
Create a string out of the map for controller tags flag
*/}}
{{- define "aws-efs-csi-driver-app.tags" -}}
{{- $tags := list -}}
{{ range $key, $val := . }}
{{- $tags = print $key ":" $val | append $tags -}}
{{- end -}}
{{- join " " $tags -}}
{{- end -}}

{{/*
Get list of all provided OIDC domains
*/}}
{{- define "oidcDomains" -}}
{{- $oidcDomains := list .Values.oidcDomain -}}
{{- if .Values.oidcDomains -}}
{{- $oidcDomains = concat $oidcDomains .Values.oidcDomains -}}
{{- end -}}
{{- compact $oidcDomains | uniq | toJson -}}
{{- end -}}
