#!/usr/bin/env bash
set -euo pipefail

# Post-processing script for helm chart generation
# Applies customizations that are lost when kubebuilder helm/v2-alpha regenerates templates

# Track upstream kubebuilder issues:
# - #5486: replicas hardcoded (not templated from values)
# - #5489: env var list format not ergonomic
# When these are fixed upstream, remove corresponding sections below.

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
# Use Python for more robust multi-line insertion
echo "  - Adding cert-manager volumeMounts and volumes to manager.yaml..."
python3 - <<'PYTHON'
import sys
import re

# Read the file
with open('dist/chart/templates/manager/manager.yaml', 'r') as f:
    content = f.read()

# Strategy: Find the securityContext block at container level (20 spaces indentation)
# and insert volumeMounts AFTER its closing {{- end }}

# Pattern: Find "securityContext:" at container level (20 spaces)
# Then find the matching {{- end }} that closes it
# Insert volumeMounts after that {{- end }}

volumemounts_block = """                  {{- if .Values.certManager.enable }}
                  volumeMounts:
                    - mountPath: /tmp/k8s-webhook-server/serving-certs
                      name: cert
                      readOnly: true
                  {{- end }}
"""

volumes_block = """            {{- if .Values.certManager.enable }}
            volumes:
              - name: cert
                secret:
                  defaultMode: 420
                  secretName: {{ include "team-operator.resourceName" (dict "suffix" "webhook-server-cert" "context" $) }}
            {{- end }}
"""

# Find securityContext at container level and insert volumeMounts after its closing
# Look for the pattern where securityContext closes with {{- end }}
# The securityContext block is at 20 spaces indentation
lines = content.split('\n')
output_lines = []
volumemounts_inserted = False
volumes_inserted = False

i = 0
while i < len(lines):
    line = lines[i]
    output_lines.append(line)

    # Insert volumeMounts after the closing of container securityContext
    # Pattern: Look for "{{- end }}" at 20 spaces (container level) that closes securityContext
    if not volumemounts_inserted and line == '                    {{- end }}':
        # Check if this closes the securityContext block by looking back
        # Look for "securityContext:" within the last 10 lines
        found_securityContext = False
        for j in range(max(0, i-10), i):
            if 'securityContext:' in lines[j] and lines[j].startswith('                  '):
                found_securityContext = True
                break

        if found_securityContext:
            # Insert volumeMounts after this closing
            output_lines.append(volumemounts_block.rstrip())
            volumemounts_inserted = True

    # Insert volumes after terminationGracePeriodSeconds
    if not volumes_inserted and 'terminationGracePeriodSeconds: 10' in line:
        output_lines.append(volumes_block.rstrip())
        volumes_inserted = True

    i += 1

# Write the file
with open('dist/chart/templates/manager/manager.yaml', 'w') as f:
    f.write('\n'.join(output_lines))

if not volumemounts_inserted:
    print("WARNING: volumeMounts not inserted!", file=sys.stderr)
    sys.exit(1)
if not volumes_inserted:
    print("WARNING: volumes not inserted!", file=sys.stderr)
    sys.exit(1)

print("Successfully inserted volumeMounts and volumes", file=sys.stderr)
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

# Issue 6: Add namespace-scoped Role to manager-role.yaml
echo "  - Adding namespace-scoped Role to manager-role.yaml..."
cat >> "$CHART_DIR/templates/rbac/manager-role.yaml" <<'EOF'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
    labels:
        app.kubernetes.io/component: rbac
        app.kubernetes.io/created-by: team-operator
        app.kubernetes.io/instance: manager-role
        app.kubernetes.io/managed-by: {{ .Release.Service }}
        app.kubernetes.io/name: role
        helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
        app.kubernetes.io/part-of: team-operator
    name: {{ include "team-operator.resourceName" (dict "suffix" "manager-role" "context" $) }}
    namespace: {{ .Values.watchNamespace }}
rules:
    - apiGroups:
        - ""
      resources:
        - configmaps
        - persistentvolumeclaims
        - pods
        - pods/attach
        - pods/exec
        - secrets
        - serviceaccounts
        - services
      verbs:
        - create
        - delete
        - get
        - list
        - patch
        - update
        - watch
    - apiGroups:
        - ""
      resources:
        - events
      verbs:
        - watch
    - apiGroups:
        - ""
      resources:
        - pods/log
      verbs:
        - get
        - list
        - watch
    - apiGroups:
        - apps
      resources:
        - daemonsets
        - deployments
        - statefulsets
      verbs:
        - create
        - delete
        - get
        - list
        - patch
        - update
        - watch
    - apiGroups:
        - batch
      resources:
        - jobs
      verbs:
        - create
        - delete
        - get
        - list
        - patch
        - update
        - watch
    - apiGroups:
        - core.posit.team
      resources:
        - chronicles
        - connects
        - flightdecks
        - packagemanagers
        - postgresdatabases
        - sites
        - workbenches
      verbs:
        - create
        - delete
        - get
        - list
        - patch
        - update
        - watch
    - apiGroups:
        - core.posit.team
      resources:
        - chronicles/finalizers
        - connects/finalizers
        - flightdecks/finalizers
        - packagemanagers/finalizers
        - postgresdatabases/finalizers
        - sites/finalizers
        - workbenches/finalizers
      verbs:
        - update
    - apiGroups:
        - core.posit.team
      resources:
        - chronicles/status
        - connects/status
        - flightdecks/status
        - packagemanagers/status
        - postgresdatabases/status
        - sites/status
        - workbenches/status
      verbs:
        - get
        - patch
        - update
    - apiGroups:
        - k8s.keycloak.org
      resources:
        - keycloakrealmimports
        - keycloaks
      verbs:
        - create
        - delete
        - get
        - list
        - patch
        - update
        - watch
    - apiGroups:
        - metrics.k8s.io
      resources:
        - pods
      verbs:
        - get
    - apiGroups:
        - networking.k8s.io
      resources:
        - ingresses
        - networkpolicies
      verbs:
        - create
        - delete
        - get
        - list
        - patch
        - update
        - watch
    - apiGroups:
        - policy
      resources:
        - poddisruptionbudgets
      verbs:
        - create
        - delete
        - get
        - list
        - patch
        - update
        - watch
    - apiGroups:
        - rbac.authorization.k8s.io
      resources:
        - rolebindings
        - roles
      verbs:
        - create
        - delete
        - get
        - list
        - patch
        - update
        - watch
    - apiGroups:
        - secrets-store.csi.x-k8s.io
      resources:
        - secretproviderclasses
        - secretsproviderclass
      verbs:
        - create
        - delete
        - get
        - list
        - patch
        - update
        - watch
    - apiGroups:
        - traefik.io
      resources:
        - middlewares
      verbs:
        - create
        - delete
        - get
        - list
        - patch
        - update
        - watch
EOF

# Issue 7: Add namespace-scoped RoleBinding to manager-rolebinding.yaml
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

# Issue 8: Add migration guide to README.md
echo "  - Adding migration guide to README.md..."
$SED -i '/^> ```$/a\
\
## Upgrading from helm\/v1-alpha\
\
If upgrading from chart versions prior to 2.0.0 (which used kubebuilder helm\/v1-alpha):\
\
### Values Changes\
\
| Old Path | New Path | Notes |\
|----------|----------|-------|\
| `controllerManager.*` | `manager.*` | Top-level key renamed |\
| `controllerManager.container.image` | `manager.image` | Flattened one level |\
| `controllerManager.container.env` | `manager.env` | Changed from map to list format |\
| `controllerManager.container.args` | `manager.args` | Flattened one level |\
| `controllerManager.container.resources` | `manager.resources` | Flattened one level |\
| `controllerManager.serviceAccountName` | _(removed)_ | Now generated from release name |\
| `controllerManager.serviceAccount.annotations` | `manager.serviceAccount.annotations` | Path changed |\
| `controllerManager.tolerations` | `manager.tolerations` | Path changed |\
| `controllerManager.nodeSelector` | `manager.nodeSelector` | Path changed |\
| `certmanager.enable` | `certManager.enable` | Capitalization changed |\
| `watchNamespace` | `watchNamespace` | Unchanged |\
| `rbac.enable` | _(removed)_ | RBAC is always enabled |\
| `webhook.enable` | _(removed)_ | Not currently used |\
| `networkPolicy.enable` | _(removed)_ | Not currently used |\
\
### Environment Variables\
\
Environment variables changed from map format to list format:\
\
**Before:**\
```yaml\
controllerManager:\
  container:\
    env:\
      WATCH_NAMESPACES: "posit-team"\
      AWS_REGION: "us-east-1"\
```\
\
**After:**\
```yaml\
manager:\
  env:\
    - name: WATCH_NAMESPACES\
      value: "posit-team"\
    - name: AWS_REGION\
      value: "us-east-1"\
```\
\
### Removed Features\
\
- **RBAC helper roles**: Convenience ClusterRoles for CRD access (e.g., `site-editor-role`, `connect-viewer-role`) are no longer generated. Create custom RBAC rules if needed.\
- **Digest-based image tags**: The `@sha256:` image tag format is no longer supported. Use standard `repository:tag` format.' "$CHART_DIR/README.md"

echo "Post-generation customizations complete!"
