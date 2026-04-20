{{- define "signer.name" -}}signer{{- end }}

{{- define "signer.fullname" -}}
{{- printf "%s-signer" .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "signer.labels" -}}
app.kubernetes.io/name: {{ include "signer.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "signer.selectorLabels" -}}
app.kubernetes.io/name: {{ include "signer.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
