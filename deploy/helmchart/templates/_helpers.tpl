{{/* Common helpers for alerthub chart */}}
{{- define "alerthub.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "alerthub.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "alerthub.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "alerthub.labels" -}}
{{ include "alerthub.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "alerthub.selectorLabels" -}}
app.kubernetes.io/name: {{ include "alerthub.name" . }}
{{- end }}

{{- define "alerthub.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "alerthub.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/* Build a component-scoped name like <fullname>-<component> */}}
{{- define "alerthub.componentName" -}}
{{- printf "%s-%s" .root.Release.Name .component | trunc 63 | trimSuffix "-" -}}
{{- end -}}
