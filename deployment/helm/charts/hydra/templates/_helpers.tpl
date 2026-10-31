{{/*
secretKeyRef of the secret of one OAuth2 client (DCS-NFR-SEC-12): its
client_secretSecretRef, else the key client_id of the chart Secret
<release>-hydra-clients. The parent chart calls it for HYDRA_CLIENT_SECRET.
Call: include "hydra.clientSecretKeyRef" (dict "release" .Release.Name "client" $client)
*/}}
{{- define "hydra.clientSecretKeyRef" -}}
{{- $ref := .client.client_secretSecretRef | default dict -}}
{{- if $ref.name -}}
name: {{ $ref.name | quote }}
key: {{ required (printf "hydra.clients[%s].client_secretSecretRef.key is required" .client.client_id) $ref.key | quote }}
{{- else -}}
{{- if not .client.client_secret -}}
{{- fail (printf "hydra.clients[%s]: set client_secretSecretRef (name, key) or client_secret" .client.client_id) -}}
{{- end -}}
name: {{ printf "%s-hydra-clients" .release | quote }}
key: {{ .client.client_id | quote }}
{{- end -}}
{{- end }}

{{/*
"true" when Hydra keeps its state in memory: no DSN Secret, no co-deployed
PostgreSQL and the literal DSN "memory".
*/}}
{{- define "hydra.inMemory" -}}
{{- if and (not .Values.config.dsnSecretRef.name) (not .Values.config.coDeployedPostgresql.enabled) (eq (toString .Values.config.dsn) "memory") -}}
true
{{- end -}}
{{- end }}
