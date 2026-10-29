{{/*
资源命名：服务之间按固定名字互相访问（配置里写 hello-service:9100），所以资源名不带 release 前缀。
workload 名为 server 时 = 服务名，其他为 <服务名>-<workload>。
*/}}
{{- define "beidou-layout.fullname" -}}
{{- if eq .workload "server" -}}
{{ .service }}
{{- else -}}
{{ .service }}-{{ .workload }}
{{- end -}}
{{- end }}

{{/* 部署命名空间：values.namespace，未设置时回退到 helm -n */}}
{{- define "beidou-layout.namespace" -}}
{{ .Values.namespace | default .Release.Namespace }}
{{- end }}

{{- define "beidou-layout.chart" -}}
{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- end }}

{{/* 选择器标签（创建后不可变，只放稳定字段） */}}
{{- define "beidou-layout.selectorLabels" -}}
app.kubernetes.io/name: {{ include "beidou-layout.fullname" . }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
{{- end }}

{{/* 通用标签：参数 dict root / service / workload */}}
{{- define "beidou-layout.labels" -}}
{{ include "beidou-layout.selectorLabels" . }}
app.kubernetes.io/component: {{ .service }}
app.kubernetes.io/part-of: beidou-layout
app.kubernetes.io/version: {{ include "beidou-layout.tag" .root | quote }}
app.kubernetes.io/managed-by: {{ .root.Release.Service }}
helm.sh/chart: {{ include "beidou-layout.chart" .root }}
{{- end }}

{{- define "beidou-layout.tag" -}}
{{ .Values.image.tag | default .Chart.AppVersion }}
{{- end }}

{{- define "beidou-layout.image" -}}
{{ .root.Values.image.registry }}/{{ .image }}:{{ include "beidou-layout.tag" .root }}
{{- end }}

{{/*
服务的配置变量：config < services.<服务>.config，另加 namespace / release。
参数 dict root / service；输出 YAML，调用方 fromYaml。
*/}}
{{- define "beidou-layout.vars" -}}
{{- $svc := index .root.Values.services .service -}}
{{- $vars := mergeOverwrite (deepCopy .root.Values.config) (deepCopy ($svc.config | default dict)) -}}
{{- $_ := set $vars "namespace" (include "beidou-layout.namespace" .root) -}}
{{- $_ := set $vars "release" .root.Release.Name -}}
{{- toYaml $vars -}}
{{- end }}

{{/* 服务配置文件：files/config/<服务>/* 渲染结果（ConfigMap data 与 checksum 共用） */}}
{{- define "beidou-layout.configData" -}}
{{- $vars := include "beidou-layout.vars" . | fromYaml -}}
{{- range $path, $content := .root.Files.Glob (printf "files/config/%s/*" .service) }}
{{ base $path }}: |
  {{- tpl (toString $content) $vars | nindent 2 }}
{{- end }}
{{- end }}

{{- define "beidou-layout.hasConfig" -}}
{{- if .root.Files.Glob (printf "files/config/%s/*" .service) }}true{{ end -}}
{{- end }}

{{/*
合并后的 workload：defaults < services.<服务>.workloads.<名字>；附带 root / service / workload / image。
参数 dict root / service / workload；输出 YAML（不含 root），调用方 fromYaml 后再 set root。
*/}}
{{- define "beidou-layout.workload" -}}
{{- $svc := index .root.Values.services .service -}}
{{- $w := mergeOverwrite (deepCopy .root.Values.defaults) (deepCopy (index $svc.workloads .workload)) -}}
{{- $_ := set $w "service" .service -}}
{{- $_ := set $w "workload" .workload -}}
{{- $_ := set $w "image" ($svc.image | default .service) -}}
{{- toYaml $w -}}
{{- end }}

{{/* 第一个端口（按名字排序）：做探针 */}}
{{- define "beidou-layout.probePort" -}}
{{- $first := "" -}}
{{- range $name, $port := .ports }}{{ if not $first }}{{ $first = $port }}{{ end }}{{ end -}}
{{ $first }}
{{- end }}

{{/* Pod 模板；参数为合并后的 workload（含 root） */}}
{{- define "beidou-layout.podSpec" -}}
{{- $root := .root -}}
{{- $hasConfig := include "beidou-layout.hasConfig" (dict "root" $root "service" .service) -}}
{{- $vars := include "beidou-layout.vars" (dict "root" $root "service" .service) | fromYaml -}}
{{- with $root.Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
# 业务不访问 k8s API；不注入 Service 环境变量（避免与 PORT 之类的变量冲突）
automountServiceAccountToken: false
enableServiceLinks: false
terminationGracePeriodSeconds: {{ .terminationGracePeriodSeconds }}
{{- with .podSecurityContext }}
securityContext:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- if gt (int .replicas) 1 }}
# 多副本尽量分散到不同节点
topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: kubernetes.io/hostname
    whenUnsatisfiable: ScheduleAnyway
    labelSelector:
      matchLabels:
        {{- include "beidou-layout.selectorLabels" (dict "root" $root "service" .service "workload" .workload) | nindent 8 }}
{{- end }}
{{- if .migrate }}
# 启动前跑数据库迁移（gormigrate，幂等）；失败则新 Pod 起不来，旧版本继续服务
initContainers:
  - name: migrate
    image: {{ include "beidou-layout.image" (dict "root" $root "image" .image) }}
    imagePullPolicy: {{ $root.Values.image.pullPolicy }}
    args: ["./{{ .image }}", "migrate", "-c", "configs/config.yaml"]
    env:
      - { name: TZ, value: {{ $root.Values.timezone | quote }} }
    {{- with .securityContext }}
    securityContext:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    volumeMounts:
      - { name: config, mountPath: /app/configs, readOnly: true }
{{- end }}
containers:
  - name: {{ .workload }}
    image: {{ include "beidou-layout.image" (dict "root" $root "image" .image) }}
    imagePullPolicy: {{ $root.Values.image.pullPolicy }}
    {{- with .args }}
    args: {{ toJson . }}
    {{- end }}
    env:
      - { name: TZ, value: {{ $root.Values.timezone | quote }} }
      {{- range $k, $v := .env }}
      - name: {{ $k }}
        value: {{ tpl (toString $v) $vars | quote }}
      {{- end }}
    {{- if .ports }}
    ports:
      {{- range $name, $port := .ports }}
      - { name: {{ $name }}, containerPort: {{ $port }}, protocol: TCP }
      {{- end }}
    {{- $probePort := include "beidou-layout.probePort" . }}
    startupProbe:
      tcpSocket: { port: {{ $probePort }} }
      periodSeconds: 2
      failureThreshold: 60
    readinessProbe:
      tcpSocket: { port: {{ $probePort }} }
      periodSeconds: 10
    livenessProbe:
      tcpSocket: { port: {{ $probePort }} }
      periodSeconds: 20
      failureThreshold: 3
    lifecycle:
      preStop:
        sleep:
          seconds: {{ .preStopSleepSeconds }}
    {{- end }}
    resources:
      {{- toYaml .resources | nindent 6 }}
    {{- with .securityContext }}
    securityContext:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- if $hasConfig }}
    volumeMounts:
      - { name: config, mountPath: /app/configs, readOnly: true }
    {{- end }}
{{- if $hasConfig }}
volumes:
  - name: config
    configMap:
      name: {{ .service }}-config
{{- end }}
{{- end }}

{{/* 遍历全部 workload，输出 list（每项为合并后的 workload，含 root） */}}
{{- define "beidou-layout.workloads" -}}
{{- $out := list -}}
{{- range $service, $svc := .Values.services -}}
{{- range $workload, $_ := $svc.workloads -}}
{{- $w := include "beidou-layout.workload" (dict "root" $ "service" $service "workload" $workload) | fromYaml -}}
{{- $out = append $out $w -}}
{{- end -}}
{{- end -}}
{{- toYaml (dict "items" $out) -}}
{{- end }}
