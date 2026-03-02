package core

import (
	"fmt"
	"strings"

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
	ppmAuthCurlrcPath       = "/mnt/ppm-auth/.curlrc"
	ppmAuthTokenMountPath   = "/var/run/secrets/ppm-auth"
	ppmAuthScriptMountPath  = "/scripts"
	ppmAuthDefaultImage     = "alpine:3"
	ppmAuthDefaultRefresh   = "3000" // 50 minutes (for 60 min token lifetime)
)

// PPMAuthTokenExchangeScript returns the shell script content for the token exchange
// init container and sidecar. The script exchanges a K8s service account token for a
// PPM API token via RFC 8693 token exchange, then writes a netrc file and a curlrc
// file (so R's libcurl can also authenticate via --netrc-file).
func PPMAuthTokenExchangeScript() string {
	return `#!/bin/sh
set -e

# Install required tools (alpine base image does not include curl or jq)
apk add --no-cache curl jq >/dev/null 2>&1

SA_TOKEN_PATH="${SA_TOKEN_PATH:-/var/run/secrets/ppm-auth/token}"
NETRC_PATH="${NETRC_PATH:-/mnt/ppm-auth/netrc}"
CURLRC_PATH="${CURLRC_PATH:-/mnt/ppm-auth/.curlrc}"
PPM_URL="${PPM_URL}"
REFRESH_INTERVAL="${REFRESH_INTERVAL:-3000}"

exchange_token() {
    SA_TOKEN=$(cat "$SA_TOKEN_PATH")
    RESPONSE=$(curl -sf -X POST "${PPM_URL}/__api__/token" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
        -d "subject_token=${SA_TOKEN}" \
        -d "subject_token_type=urn:ietf:params:oauth:token-type:id_token")

    PPM_TOKEN=$(echo "$RESPONSE" | jq -r '.access_token')

    if [ -z "$PPM_TOKEN" ] || [ "$PPM_TOKEN" = "null" ]; then
        echo "ERROR: Failed to extract access_token from PPM response" >&2
        exit 1
    fi

    PPM_HOST=$(echo "$PPM_URL" | sed 's|https\?://||' | sed 's|/.*||')

    # Write netrc (atomic: write to temp, then rename)
    TMPFILE=$(mktemp "${NETRC_PATH}.XXXXXX")
    printf "machine %s\nlogin __token__\npassword %s\n" "$PPM_HOST" "$PPM_TOKEN" > "$TMPFILE"
    mv "$TMPFILE" "$NETRC_PATH"
    chmod 600 "$NETRC_PATH"

    # Write curlrc so R's libcurl uses the netrc file
    printf -- "--netrc-file %s\n" "$NETRC_PATH" > "$CURLRC_PATH"
    chmod 600 "$CURLRC_PATH"
}

exchange_token

if [ "${MODE}" = "sidecar" ]; then
    while true; do
        sleep "$REFRESH_INTERVAL"
        exchange_token
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
			AllowPrivilegeEscalation: ptr.To(false),
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
func PPMAuthVolumes(siteName, ppmURL string) []corev1.Volume {
	// Extract audience from PPM URL (the PPM URL itself is the audience for the projected token)
	audience := ppmURL

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
func SanitizePPMUrl(rawUrl string) string {
	host := strings.TrimPrefix(strings.TrimPrefix(rawUrl, "https://"), "http://")
	return fmt.Sprintf("https://%s", host)
}
