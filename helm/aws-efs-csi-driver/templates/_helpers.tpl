{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "labels.common" -}}
app.kubernetes.io/managed-by: {{ .Release.Service | quote }}
app.kubernetes.io/name: {{ .Values.name | quote }}
app.kubernetes.io/instance: {{ .Release.Name | quote }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
giantswarm.io/service-type: "managed"
helm.sh/chart: {{ include "chart" . | quote }}
{{- end -}}

{{/*
Create a string out of the map for controller tags flag
*/}}
{{- define "aws-efs-csi-driver.tags" -}}
{{- $tags := list -}}
{{ range $key, $val := . }}
{{- $tags = print $key ":" $val | append $tags -}}
{{- end -}}
{{- join " " $tags -}}
{{- end -}}