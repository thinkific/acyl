{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "fullname" -}}
{{- printf "%s" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create .Data.value template for Vault Agent Injector
Note the newline \n and spaces to maintain indentation
*/}}
{{- define "vault-agent-default-secrets-v1-template" -}}
{{- $secrets_prefix := .Values.vault.secrets_prefix -}}
{{- range .Values.vault.secrets }}
vault.hashicorp.com/agent-inject-secret-{{ .name }}: {{- printf " \"%v%v\"\n" $secrets_prefix .path -}}
vault.hashicorp.com/agent-inject-template-{{- printf "%v: |\n" .name  -}}
{{- printf "\n  {{- with secret \"%v%v\" -}}\n" $secrets_prefix .path  -}}
{{- printf "  {{ .Data.value }}\n" -}}  
{{- println "  {{- end }}" -}}
{{- end -}}
{{- end -}}
