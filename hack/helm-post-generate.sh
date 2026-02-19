#!/usr/bin/env bash
set -euo pipefail

# Post-processing script for helm chart generation
# Applies customizations that are lost when kubebuilder helm/v2-alpha regenerates templates

CHART_DIR="dist/chart"
SED="${SED:-sed}"

echo "Applying post-generation customizations to Helm chart..."

# Issue 1: Fix values.yaml - remove duplicate args that are now handled by manager.yaml template
echo "  - Fixing values.yaml args..."
# Remove the metrics-bind-address and health-probe-bind-address lines (now handled by manager.yaml template)
$SED -i '/--metrics-bind-address=:8443/d' "$CHART_DIR/values.yaml"
$SED -i '/--health-probe-bind-address=:8081/d' "$CHART_DIR/values.yaml"

# Issue 2: Fix manager.yaml - template the replicas field
echo "  - Templating replicas in manager.yaml..."
$SED -i 's/^    replicas: 1$/    replicas: {{ .Values.manager.replicas }}/' "$CHART_DIR/templates/manager/manager.yaml"

# Issue 3: Add imagePullSecrets support to manager.yaml
# Insert after "        spec:" line (the pod spec, which is indented 8 spaces)
echo "  - Adding imagePullSecrets to manager.yaml..."
$SED -i '/^        spec:$/a\
            {{- with .Values.manager.imagePullSecrets }}\
            imagePullSecrets: {{ toYaml . | nindent 12 }}\
            {{- end }}' "$CHART_DIR/templates/manager/manager.yaml"

# Issue 4: Add cert-manager volumeMounts and volumes to manager.yaml
# Use Python for more complex multi-line insertion
echo "  - Adding cert-manager volumeMounts and volumes to manager.yaml..."
python3 - <<'PYTHON'
import sys

# Read the file
with open('dist/chart/templates/manager/manager.yaml', 'r') as f:
    lines = f.readlines()

# Find where to insert volumeMounts (after the last line of securityContext block in container)
# Find where to insert volumes (after terminationGracePeriodSeconds)
output = []
i = 0
volumemounts_inserted = False
volumes_inserted = False

while i < len(lines):
    line = lines[i]
    output.append(line)

    # Insert volumeMounts after the container's securityContext block closes
    # Look for the pattern: securityContext block followed by either another top-level container field or pod-level field
    if not volumemounts_inserted and line.strip() == '{}' and i > 0:
        # Check if this is the closing of container securityContext (look back for "securityContext:")
        # and check if next non-empty line is at the pod level (less indentation)
        if i + 1 < len(lines):
            # Check previous context - should have "securityContext:" before
            found_sec_ctx = False
            for j in range(max(0, i-5), i):
                if 'securityContext:' in lines[j] and lines[j].startswith('                  '):  # container level (18 spaces)
                    found_sec_ctx = True
                    break

            if found_sec_ctx:
                # Insert volumeMounts here
                output.append('                  {{- if .Values.certManager.enable }}\n')
                output.append('                  volumeMounts:\n')
                output.append('                    - mountPath: /tmp/k8s-webhook-server/serving-certs\n')
                output.append('                      name: cert\n')
                output.append('                      readOnly: true\n')
                output.append('                  {{- end }}\n')
                volumemounts_inserted = True

    # Insert volumes after terminationGracePeriodSeconds
    if not volumes_inserted and 'terminationGracePeriodSeconds: 10' in line:
        output.append('            {{- if .Values.certManager.enable }}\n')
        output.append('            volumes:\n')
        output.append('              - name: cert\n')
        output.append('                secret:\n')
        output.append('                  defaultMode: 420\n')
        output.append('                  secretName: {{ include "team-operator.resourceName" (dict "suffix" "webhook-server-cert" "context" $) }}\n')
        output.append('            {{- end }}\n')
        volumes_inserted = True

    i += 1

# Write the file
with open('dist/chart/templates/manager/manager.yaml', 'w') as f:
    f.writelines(output)

if not volumemounts_inserted:
    print("WARNING: volumeMounts not inserted!", file=sys.stderr)
    sys.exit(1)
if not volumes_inserted:
    print("WARNING: volumes not inserted!", file=sys.stderr)
    sys.exit(1)
PYTHON

# Issue 5: Add ServiceAccount annotations to controller-manager.yaml
echo "  - Adding ServiceAccount annotations to controller-manager.yaml..."
$SED -i '/^    labels:$/i\
    {{- with .Values.manager.serviceAccount }}\
    {{- with .annotations }}\
    annotations:\
      {{- toYaml . | nindent 6 }}\
    {{- end }}\
    {{- end }}' "$CHART_DIR/templates/rbac/controller-manager.yaml"

# Issue 6: Add namespace-scoped RoleBinding to manager-rolebinding.yaml
echo "  - Adding namespace-scoped RoleBinding to manager-rolebinding.yaml..."
cat >> "$CHART_DIR/templates/rbac/manager-rolebinding.yaml" <<'EOF'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
    labels:
        app.kubernetes.io/component: rbac
        app.kubernetes.io/created-by: team-operator
        app.kubernetes.io/instance: manager-rolebinding
        app.kubernetes.io/managed-by: {{ .Release.Service }}
        app.kubernetes.io/name: rolebinding
        helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
        app.kubernetes.io/part-of: team-operator
    name: {{ include "team-operator.resourceName" (dict "suffix" "manager-rolebinding" "context" $) }}
    namespace: {{ .Values.watchNamespace }}
roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: Role
    name: {{ include "team-operator.resourceName" (dict "suffix" "manager-role" "context" $) }}
subjects:
    - kind: ServiceAccount
      name: {{ include "team-operator.resourceName" (dict "suffix" "controller-manager" "context" $) }}
      namespace: {{ .Release.Namespace }}
EOF

echo "Post-generation customizations complete!"
