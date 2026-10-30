{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "aws-fsx-openzfs-csi-driver.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "aws-fsx-openzfs-csi-driver.fullname" -}}
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
{{- define "aws-fsx-openzfs-csi-driver.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "aws-fsx-openzfs-csi-driver.labels" -}}
{{ include "aws-fsx-openzfs-csi-driver.selectorLabels" . }}
{{- if ne .Release.Name "kustomize" }}
helm.sh/chart: {{ include "aws-fsx-openzfs-csi-driver.chart" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
{{- end -}}

{{/*
Common selector labels
*/}}
{{- define "aws-fsx-openzfs-csi-driver.selectorLabels" -}}
app.kubernetes.io/part-of: {{ include "aws-fsx-openzfs-csi-driver.name" . }}
{{- if ne .Release.Name "kustomize" }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
{{- end -}}

{{/*
Controller arguments for the cluster tag, cluster tag isolation and extra tags.
*/}}
{{- define "aws-fsx-openzfs-csi-driver.clusterTagArgs" -}}
{{- $c := .Values.controller -}}
{{- $name := trim (toString (default "" $c.clusterName)) -}}
{{- $key := trim (toString (default "" $c.clusterTagKey)) -}}
{{- $isolation := $c.clusterTagIsolation -}}
{{- if kindIs "invalid" $isolation -}}
{{- $isolation = "disable" -}}
{{- end -}}
{{- if not (kindIs "string" $isolation) -}}
{{- fail (printf "controller.clusterTagIsolation must be one of \"disable\", \"backfill\" or \"enforce\", got the %s %v" (kindOf $isolation) $isolation) -}}
{{- end -}}
{{- if not (has $isolation (list "" "disable" "backfill" "enforce")) -}}
{{- fail (printf "controller.clusterTagIsolation must be \"disable\", \"backfill\" or \"enforce\", got %q" $isolation) -}}
{{- end -}}
{{- if eq $isolation "" -}}
{{- $isolation = "disable" -}}
{{- end -}}
{{- if and (ne $isolation "disable") (or (not $name) (not $key)) -}}
{{- fail (printf "controller.clusterTagIsolation=%s requires controller.clusterName and controller.clusterTagKey" $isolation) -}}
{{- end -}}
{{- if $name }}
- {{ printf "--cluster-name=%s" $name | quote }}
{{- end }}
{{- if $key }}
- {{ printf "--cluster-tag-key=%s" $key | quote }}
{{- end }}
{{- if ne $isolation "disable" }}
- --cluster-tag-isolation={{ $isolation }}
{{- end }}
{{- with $c.extraTags }}
{{- $tags := list -}}
{{- range $k, $v := . -}}
{{- $tags = append $tags (printf "%s=%s" $k (toString $v)) -}}
{{- end }}
- {{ printf "--extra-tags=%s" (join "," $tags) | quote }}
{{- end }}
{{- end -}}
