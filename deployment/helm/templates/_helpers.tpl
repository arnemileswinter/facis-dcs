{{/*
Expand the name of the chart.
*/}}
{{- define "digital-contracting-service.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "digital-contracting-service.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "digital-contracting-service.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "digital-contracting-service.labels" -}}
helm.sh/chart: {{ include "digital-contracting-service.chart" . }}
{{ include "digital-contracting-service.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "digital-contracting-service.selectorLabels" -}}
app.kubernetes.io/name: {{ include "digital-contracting-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "digital-contracting-service.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "digital-contracting-service.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Normalize a route base path to always start with "/" and never end with "/".
*/}}
{{- define "digital-contracting-service.baseRoutePath" -}}
{{- $base := default "digital-contracting-service" .Values.route.basePath -}}
{{- printf "/%s" (trimAll "/" ($base | toString)) -}}
{{- end }}

{{/*
Resolve PostgreSQL host (explicit override or in-chart default).
*/}}
{{- define "digital-contracting-service.postgresqlHost" -}}
{{- if .Values.serviceDiscovery.postgresqlHost -}}
{{- .Values.serviceDiscovery.postgresqlHost -}}
{{- else if .Values.postgresql.enabled -}}
{{- printf "%s-postgresql" .Release.Name -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end }}

{{/*
Resolve Hydra host (explicit override or in-chart default).
*/}}
{{- define "digital-contracting-service.hydraHost" -}}
{{- if .Values.serviceDiscovery.hydraHost -}}
{{- .Values.serviceDiscovery.hydraHost -}}
{{- else if .Values.hydra.enabled -}}
{{- printf "%s-hydra" .Release.Name -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end }}

{{/*
Resolve NATS host (explicit override or in-chart default).
*/}}
{{- define "digital-contracting-service.natsHost" -}}
{{- if .Values.serviceDiscovery.natsHost -}}
{{- .Values.serviceDiscovery.natsHost -}}
{{- else if .Values.nats.enabled -}}
{{- printf "%s-nats" .Release.Name -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end }}

{{/*
Resolve Hydra public port from explicit override or in-chart service.
*/}}
{{- define "digital-contracting-service.hydraPublicPort" -}}
{{- if .Values.serviceDiscovery.hydraPublicPort -}}
{{- .Values.serviceDiscovery.hydraPublicPort -}}
{{- else if .Values.hydra.enabled -}}
{{- default 4444 .Values.hydra.service.publicPort -}}
{{- else if eq (default "http" .Values.oidc.scheme) "https" -}}
443
{{- else -}}
80
{{- end -}}
{{- end }}

{{/*
DATABASE_URL override or derived from postgres settings.
*/}}
{{- define "digital-contracting-service.databaseURL" -}}
{{- if .Values.database.url -}}
{{- .Values.database.url -}}
{{- else if include "digital-contracting-service.postgresqlHost" . -}}
{{- $host := include "digital-contracting-service.postgresqlHost" . -}}
{{- $port := default 5432 .Values.database.port -}}
{{- $user := default (default "dcs" .Values.postgresql.auth.username) .Values.database.user -}}
{{- $password := default (default "dcs" .Values.postgresql.auth.password) .Values.database.password -}}
{{- $dbname := default (default "dcs" .Values.postgresql.auth.database) .Values.database.name -}}
{{- $sslmode := default "disable" .Values.database.sslmode -}}
{{- printf "host=%s port=%v user=%s password=%s dbname=%s sslmode=%s" $host $port $user $password $dbname $sslmode -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end }}

{{/*
NATS_URL override or derived from nats settings.
*/}}
{{- define "digital-contracting-service.natsURL" -}}
{{- if .Values.messaging.natsURL -}}
{{- .Values.messaging.natsURL -}}
{{- else if include "digital-contracting-service.natsHost" . -}}
{{- $host := include "digital-contracting-service.natsHost" . -}}
{{- $port := default 4222 .Values.messaging.natsPort -}}
{{- printf "nats://%s:%v" $host $port -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end }}

{{/*
OIDC issuer override or derived from Hydra settings.
Uses external URL (istio/ingress host) for browser-based OIDC flows.
*/}}
{{- define "digital-contracting-service.oidcIssuerURL" -}}
{{- if .Values.oidc.issuerURL -}}
{{- .Values.oidc.issuerURL -}}
{{- else if and .Values.hydra.enabled .Values.hydra.route.path -}}
{{- $scheme := default "https" .Values.oidc.scheme -}}
{{- $basePath := printf "/%s" (trimAll "/" .Values.hydra.route.path) -}}
{{- $host := "" -}}
{{- if and .Values.istio.enabled (gt (len .Values.istio.hosts) 0) -}}
{{- $host = index .Values.istio.hosts 0 -}}
{{- else if and .Values.ingress.enabled (gt (len .Values.ingress.hosts) 0) -}}
{{- $host = (index .Values.ingress.hosts 0).host -}}
{{- end -}}
{{- if $host -}}
{{- printf "%s://%s%s" $scheme $host $basePath -}}
{{- end -}}
{{- else if .Values.hydra.enabled -}}
{{- printf "http://%s:%v" (include "digital-contracting-service.hydraHost" .) (include "digital-contracting-service.hydraPublicPort" .) -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end }}

{{/*
API path override or derived default.
*/}}
{{- define "digital-contracting-service.apiPath" -}}
{{- if .Values.paths.api -}}
{{- .Values.paths.api -}}
{{- else -}}
{{- printf "%s/api" (include "digital-contracting-service.baseRoutePath" .) -}}
{{- end -}}
{{- end }}

{{/*
UI path override or derived default.
*/}}
{{- define "digital-contracting-service.uiPath" -}}
{{- if .Values.paths.ui -}}
{{- .Values.paths.ui -}}
{{- else -}}
{{- printf "%s/ui" (include "digital-contracting-service.baseRoutePath" .) -}}
{{- end -}}
{{- end }}

{{/*
OCM_W_ISSUER_URL: explicit override or auto-wired to co-deployed issuer service.
*/}}
{{- define "digital-contracting-service.ocmWIssuerURL" -}}
{{- if .Values.ocmW.issuerURL -}}
{{- .Values.ocmW.issuerURL -}}
{{- else if .Values.ocmW.enabled -}}
{{- printf "http://%s-issuance-service:8080" .Release.Name -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end }}

{{/*
Normalize Hydra route path (leading slash, no trailing slash).
*/}}
{{- define "digital-contracting-service.hydraRoutePath" -}}
{{- if .Values.hydra.route.path -}}
{{- printf "/%s" (trimAll "/" (.Values.hydra.route.path | toString)) -}}
{{- end -}}
{{- end }}
