{{/*
Expand the name of the chart.
*/}}
{{- define "yurt-lb-agent.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "yurt-lb-agent.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "yurt-lb-agent.labels" -}}
helm.sh/chart: {{ include "yurt-lb-agent.chart" . }}
{{ include "yurt-lb-agent.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "yurt-lb-agent.selectorLabels" -}}
app.kubernetes.io/name: {{ include "yurt-lb-agent.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}