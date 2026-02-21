{{/*
Expand the name of the chart.
*/}}
{{- define "odi.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
Truncated at 63 chars because Kubernetes name fields are limited to 63 characters.
*/}}
{{- define "odi.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Backend component full name.
*/}}
{{- define "odi.backend.name" -}}
{{- printf "%s-backend" (include "odi.fullname" .) }}
{{- end }}

{{/*
Frontend component full name.
*/}}
{{- define "odi.frontend.name" -}}
{{- printf "%s-frontend" (include "odi.fullname" .) }}
{{- end }}

{{/*
Create chart label for selector.
*/}}
{{- define "odi.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every resource.
*/}}
{{- define "odi.labels" -}}
helm.sh/chart: {{ include "odi.chart" . }}
{{ include "odi.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels for the whole release (used on top-level resources).
*/}}
{{- define "odi.selectorLabels" -}}
app.kubernetes.io/name: {{ include "odi.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Selector labels for the backend Deployment / Service.
*/}}
{{- define "odi.backend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "odi.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Selector labels for the frontend Deployment / Service.
*/}}
{{- define "odi.frontend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "odi.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: frontend
{{- end }}

{{/*
Name of the main ODI secret.
*/}}
{{- define "odi.secretName" -}}
{{- printf "%s-secret" (include "odi.fullname" .) }}
{{- end }}

{{/*
Name of the PostgreSQL bootstrap secret (kubernetes.io/basic-auth).
*/}}
{{- define "odi.postgres.secretName" -}}
{{- printf "%s-postgres-secret" (include "odi.fullname" .) }}
{{- end }}

{{/*
Name of the CloudNativePG Cluster resource.
*/}}
{{- define "odi.postgres.clusterName" -}}
{{- printf "%s-postgres" (include "odi.fullname" .) }}
{{- end }}

{{/*
Read-write Service name produced by CloudNativePG.
CNPG names it <cluster-name>-rw.
*/}}
{{- define "odi.postgres.rwService" -}}
{{- printf "%s-rw" (include "odi.postgres.clusterName" .) }}
{{- end }}

{{/*
OpenSearch address.
When the subchart is enabled the service name is <release>-opensearch:9200.
*/}}
{{- define "odi.opensearchAddr" -}}
{{- if .Values.opensearch.enabled -}}
{{- printf "https://%s-opensearch:9200" .Release.Name }}
{{- else -}}
{{- required "externalOpensearch.addr is required when opensearch.enabled=false" .Values.externalOpensearch.addr }}
{{- end }}
{{- end }}

{{/*
OpenSearch admin username.
*/}}
{{- define "odi.opensearchUsername" -}}
{{- if .Values.opensearch.enabled -}}
admin
{{- else -}}
{{- required "externalOpensearch.username is required when opensearch.enabled=false" .Values.externalOpensearch.username }}
{{- end }}
{{- end }}

{{/*
OpenSearch admin password.
When using the bundled subchart the password is taken from extraEnvs[0].
*/}}
{{- define "odi.opensearchPassword" -}}
{{- if .Values.opensearch.enabled -}}
{{- index .Values.opensearch.extraEnvs 0 "value" }}
{{- else -}}
{{- required "externalOpensearch.password is required when opensearch.enabled=false" .Values.externalOpensearch.password }}
{{- end }}
{{- end }}

{{/*
Zefix PostgreSQL DSN.
When the bundled CNPG cluster is enabled the DSN is constructed from postgresql.auth.
When disabled, externalZefix.dsn must be provided.
*/}}
{{- define "odi.zefixDSN" -}}
{{- if .Values.postgresql.enabled -}}
{{- printf "postgresql://%s:%s@%s:5432/%s?sslmode=disable"
    .Values.postgresql.auth.username
    .Values.postgresql.auth.password
    (include "odi.postgres.rwService" .)
    .Values.postgresql.auth.database }}
{{- else -}}
{{- required "externalZefix.dsn is required when postgresql.enabled=false" .Values.externalZefix.dsn }}
{{- end }}
{{- end }}

{{/*
Image pull secrets block.
*/}}
{{- define "odi.imagePullSecrets" -}}
{{- with .Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}
