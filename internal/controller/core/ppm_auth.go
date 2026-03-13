package core

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/posit-dev/team-operator/api/product"
	"github.com/rstudio/goex/ptr"
)

const (
	ppmAuthScriptVolumeName = "ppm-auth-script"
	ppmAuthTokenVolumeName  = "ppm-sa-token"
	ppmAuthNetrcVolumeName  = "ppm-auth"
	ppmAuthNetrcMountPath   = "/mnt/ppm-auth"
	ppmAuthNetrcPath        = "/mnt/ppm-auth/netrc"
	ppmAuthTokenMountPath   = "/var/run/secrets/ppm-auth"
	ppmAuthScriptMountPath  = "/scripts"
	// ppmAuthDefaultImage uses alpine:3 which has wget and sed via BusyBox
	ppmAuthDefaultImage   = "alpine:3"
	ppmAuthDefaultRefresh = "3000" // 50 minutes (for 60 min token lifetime)
)

// PPMAuthTokenExchangeScript returns the shell script content for the token exchange
// init container and sidecar. The script exchanges a K8s service account token for a
// PPM API token via RFC 8693 token exchange, then writes a netrc file and a curlrc
// file (so R's libcurl can also authenticate via --netrc-file).
func PPMAuthTokenExchangeScript() string {
	return `#!/bin/sh
set -e

SA_TOKEN_PATH="${SA_TOKEN_PATH:-/var/run/secrets/ppm-auth/token}"
NETRC_PATH="${NETRC_PATH:-/mnt/ppm-auth/netrc}"
CURLRC_PATH="${CURLRC_PATH:-/mnt/ppm-auth/.curlrc}"
PPM_URL="${PPM_URL}"
REFRESH_INTERVAL="${REFRESH_INTERVAL:-3000}"

# extract_json_field extracts a string value from a JSON object using only
# shell builtins and sed. This avoids requiring jq in the container image.
# Usage: extract_json_field '{"access_token":"eyJhbG.payload.sig"}' "access_token"
#   → eyJhbG.payload.sig
# Note: assumes flat JSON with unescaped string values (no nested objects,
# no escaped quotes in values). Sufficient for OAuth token responses.
extract_json_field() {
    echo "$1" | sed -n 's/.*"'"$2"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
}

exchange_token() {
    SA_TOKEN=$(cat "$SA_TOKEN_PATH")

    # Write POST data to a temp file to avoid exposing token in process args
    POST_DATA_FILE=$(mktemp)
    trap 'rm -f "$POST_DATA_FILE"' EXIT
    printf "grant_type=urn:ietf:params:oauth:grant-type:token-exchange&subject_token=%s&subject_token_type=urn:ietf:params:oauth:token-type:id_token" "$SA_TOKEN" > "$POST_DATA_FILE"

    # Use wget (BusyBox built-in) instead of curl for zero-dependency operation
    RESPONSE=$(wget -qO- --header="Content-Type: application/x-www-form-urlencoded" \
        --post-file="$POST_DATA_FILE" \
        "${PPM_URL}/__api__/token")

    # Clean up temp file immediately (also handled by trap on failure)
    rm -f "$POST_DATA_FILE"
    trap - EXIT

    PPM_TOKEN=$(extract_json_field "$RESPONSE" "access_token")

    if [ -z "$PPM_TOKEN" ] || [ "$PPM_TOKEN" = "null" ]; then
        echo "ERROR: Failed to extract access_token from PPM response" >&2
        echo "Response length: ${#RESPONSE} bytes" >&2
        return 1
    fi

    # Sanity check: JWTs have exactly two dots (header.payload.signature)
    DOT_COUNT=$(printf '%s' "$PPM_TOKEN" | tr -cd '.' | wc -c | tr -d ' ')
    if [ "$DOT_COUNT" -ne 2 ]; then
        echo "ERROR: access_token does not look like a JWT (expected 2 dots, got $DOT_COUNT)" >&2
        return 1
    fi

    PPM_HOST=$(echo "$PPM_URL" | sed 's|https\?://||' | sed 's|/.*||')

    # Write netrc (atomic: write to temp, then rename)
    TMPFILE=$(mktemp "${NETRC_PATH}.XXXXXX") || return 1
    printf "machine %s\nlogin __token__\npassword %s\n" "$PPM_HOST" "$PPM_TOKEN" > "$TMPFILE" || return 1
    mv "$TMPFILE" "$NETRC_PATH" || return 1
    chmod 600 "$NETRC_PATH" || return 1

    # Write curlrc so R's libcurl uses the netrc file
    printf -- "--netrc-file %s\n" "$NETRC_PATH" > "$CURLRC_PATH" || return 1
    chmod 600 "$CURLRC_PATH" || return 1
}

exchange_token

if [ "${MODE}" = "sidecar" ]; then
    FAIL_COUNT=0
    MAX_FAILURES=5
    while true; do
        sleep "$REFRESH_INTERVAL"
        if exchange_token; then
            FAIL_COUNT=0
        else
            FAIL_COUNT=$((FAIL_COUNT + 1))
            if [ "$FAIL_COUNT" -ge "$MAX_FAILURES" ]; then
                echo "ERROR: token refresh failed $FAIL_COUNT consecutive times, exiting" >&2
                exit 1
            fi
            echo "WARNING: token refresh failed ($FAIL_COUNT/$MAX_FAILURES), will retry" >&2
        fi
    done
fi
`
}

// PPMAuthConfigMapName returns the name of the ConfigMap containing the token exchange script
func PPMAuthConfigMapName(siteName string) string {
	return fmt.Sprintf("%s-ppm-auth-script", siteName)
}

// PPMAuthInitContainer returns the init container spec for the PPM token exchange
func PPMAuthInitContainer(image, ppmURL string) corev1.Container {
	if image == "" {
		image = ppmAuthDefaultImage
	}
	return corev1.Container{
		Name:    "ppm-auth-init",
		Image:   image,
		Command: []string{"/scripts/token-exchange.sh"},
		Env: []corev1.EnvVar{
			{Name: "PPM_URL", Value: ppmURL},
			{Name: "MODE", Value: "init"},
		},
		VolumeMounts: ppmAuthContainerVolumeMounts(),
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             ptr.To(true),
			RunAsUser:                ptr.To(int64(65534)), // nobody
			AllowPrivilegeEscalation: ptr.To(false),
		},
	}
}

// PPMAuthSidecarContainer returns the sidecar container spec for the PPM token refresh
func PPMAuthSidecarContainer(image, ppmURL, refreshInterval string) corev1.Container {
	if image == "" {
		image = ppmAuthDefaultImage
	}
	if refreshInterval == "" {
		refreshInterval = ppmAuthDefaultRefresh
	}
	return corev1.Container{
		Name:    "ppm-auth-sidecar",
		Image:   image,
		Command: []string{"/scripts/token-exchange.sh"},
		Env: []corev1.EnvVar{
			{Name: "PPM_URL", Value: ppmURL},
			{Name: "MODE", Value: "sidecar"},
			{Name: "REFRESH_INTERVAL", Value: refreshInterval},
		},
		VolumeMounts: ppmAuthContainerVolumeMounts(),
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             ptr.To(true),
			RunAsUser:                ptr.To(int64(65534)), // nobody
			AllowPrivilegeEscalation: ptr.To(false),
		},
		// LivenessProbe restarts the sidecar if the netrc file goes stale (not modified
		// within 2x the refresh interval). This recovers from the 5-consecutive-failure exit.
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"sh", "-c",
						fmt.Sprintf("test -f %s && test $(( $(date +%%s) - $(stat -c %%Y %s 2>/dev/null || stat -f %%m %s) )) -lt %d",
							ppmAuthNetrcPath, ppmAuthNetrcPath, ppmAuthNetrcPath, ppmAuthLivenessThresholdSeconds(refreshInterval))},
				},
			},
			InitialDelaySeconds: 60,
			PeriodSeconds:       60,
			FailureThreshold:    3,
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("16Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			},
		},
	}
}

// ppmAuthLivenessThresholdSeconds returns 2x the refresh interval in seconds.
// refreshInterval must be a bare integer (seconds); values with unit suffixes
// (e.g. "30s") are treated as unparseable and fall back to the default (3000).
func ppmAuthLivenessThresholdSeconds(refreshInterval string) int {
	const defaultInterval = 3000
	val, err := strconv.Atoi(refreshInterval)
	if err != nil || val <= 0 {
		val = defaultInterval
	}
	return val * 2
}

// ppmAuthContainerVolumeMounts returns the volume mounts used by both init and sidecar containers
func ppmAuthContainerVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      ppmAuthTokenVolumeName,
			MountPath: ppmAuthTokenMountPath,
			ReadOnly:  true,
		},
		{
			Name:      ppmAuthNetrcVolumeName,
			MountPath: ppmAuthNetrcMountPath,
		},
		{
			Name:      ppmAuthScriptVolumeName,
			MountPath: ppmAuthScriptMountPath,
			ReadOnly:  true,
		},
	}
}

// PPMAuthVolumes returns the volumes needed for PPM authenticated repo access:
// 1. Projected SA token volume (for K8s Identity Federation)
// 2. Shared emptyDir for netrc file
// 3. ConfigMap volume with the token exchange script
func PPMAuthVolumes(siteName, audience string) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: ppmAuthTokenVolumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
								Path:              "token",
								ExpirationSeconds: ptr.To(int64(3600)),
								Audience:          audience,
							},
						},
					},
					DefaultMode: ptr.To(product.MustParseOctal("0644")),
				},
			},
		},
		{
			Name: ppmAuthNetrcVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: ppmAuthScriptVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: PPMAuthConfigMapName(siteName),
					},
					DefaultMode: ptr.To(product.MustParseOctal("0755")),
				},
			},
		},
	}
}

// PPMAuthVolumeMounts returns the volume mounts to add to the main product container
// for accessing the netrc file written by the init/sidecar containers
func PPMAuthVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      ppmAuthNetrcVolumeName,
			MountPath: ppmAuthNetrcMountPath,
			ReadOnly:  true,
		},
	}
}

// PPMAuthEnvVars returns the environment variables to add to the main product container
// for authenticated PPM repo access:
// - NETRC: tells Python/pip where to find the netrc file
// - CURL_HOME: tells R's libcurl where to find the .curlrc file (which references the netrc)
func PPMAuthEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "NETRC",
			Value: ppmAuthNetrcPath,
		},
		{
			Name:  "CURL_HOME",
			Value: ppmAuthNetrcMountPath,
		},
	}
}

// SanitizePPMUrl strips any existing scheme from the URL and prepends https://
// Returns an empty string if the input is empty.
func SanitizePPMUrl(rawUrl string) string {
	host := strings.TrimPrefix(strings.TrimPrefix(rawUrl, "https://"), "http://")
	if host == "" {
		return ""
	}
	return fmt.Sprintf("https://%s", host)
}

// PPMAuthSetup contains the volumes, mounts, env vars, and containers needed for PPM auth
type PPMAuthSetup struct {
	Volumes           []corev1.Volume
	VolumeMounts      []corev1.VolumeMount
	EnvVars           []corev1.EnvVar
	InitContainers    []corev1.Container
	SidecarContainers []corev1.Container
}

// SetupPPMAuth configures PPM authenticated repos for a product if enabled.
// Returns empty setup if AuthenticatedRepos is false or PPMUrl is empty.
// Logs a warning if AuthenticatedRepos is true but PPMUrl is empty.
func SetupPPMAuth(authenticatedRepos bool, ppmURL, ppmAuthImage, ppmAuthAudience, siteName string, logger logr.Logger) PPMAuthSetup {
	if !authenticatedRepos {
		return PPMAuthSetup{}
	}

	sanitizedURL := SanitizePPMUrl(ppmURL)
	if sanitizedURL == "" {
		logger.Info("AuthenticatedRepos is enabled but PPMUrl is empty; skipping PPM auth setup")
		return PPMAuthSetup{}
	}

	if ppmAuthAudience == "" {
		logger.Info("AuthenticatedRepos is enabled but PPMAuthAudience is empty; skipping PPM auth setup (projected SA token requires an audience)")
		return PPMAuthSetup{}
	}

	return PPMAuthSetup{
		Volumes:      PPMAuthVolumes(siteName, ppmAuthAudience),
		VolumeMounts: PPMAuthVolumeMounts(),
		EnvVars:      PPMAuthEnvVars(),
		InitContainers: []corev1.Container{
			PPMAuthInitContainer(ppmAuthImage, sanitizedURL),
		},
		SidecarContainers: []corev1.Container{
			PPMAuthSidecarContainer(ppmAuthImage, sanitizedURL, ""),
		},
	}
}

// UnpackPPMAuthSetup unpacks a PPMAuthSetup struct into individual slices for use in pod specs.
// This helper reduces duplication in Connect and Workbench reconcilers.
func UnpackPPMAuthSetup(setup PPMAuthSetup) (
	volumes []corev1.Volume,
	volumeMounts []corev1.VolumeMount,
	envVars []corev1.EnvVar,
	initContainers []corev1.Container,
	sidecarContainers []corev1.Container,
) {
	return setup.Volumes, setup.VolumeMounts, setup.EnvVars, setup.InitContainers, setup.SidecarContainers
}
