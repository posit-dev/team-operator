package v1beta1

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	//go:embed _assets
	assets embed.FS
)

// WorkbenchSecretConfig is a "top-level" configuration object.
// It has "child-structs" which have different config formats, and the `GenerateSecretData`
// method generates a map[string]string which can be used to create a secret with the contents
type WorkbenchSecretConfig struct {
	WorkbenchSecretIniConfig `json:"workbench-secret-ini-config,omitempty"`
}

type WorkbenchSecretIniConfig struct {
	Database           *WorkbenchDatabaseConfig              `json:"database.conf,omitempty"`
	OpenidClientSecret *WorkbenchOpenidClientSecret          `json:"openid-client-secret,omitempty"`
	Databricks         map[string]*WorkbenchDatabricksConfig `json:"databricks.conf,omitempty"`
}

func (w *WorkbenchSecretIniConfig) GenerateConfigMap() map[string]string {
	configMap := make(map[string]string, countInitializedFields(*w))

	configStructValsPtr := reflect.ValueOf(w)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		var builder strings.Builder

		fieldName := configStructVals.Type().Field(i).Name
		field, _ := reflect.TypeOf(w).Elem().FieldByName(fieldName)
		fieldTag := string(field.Tag)
		fieldTag = strings.ReplaceAll(fieldTag, "json:\"", "")
		fieldTag = strings.ReplaceAll(fieldTag, ",omitempty\"", "")
		fieldValue := configStructVals.Field(i)

		if fieldValue.IsNil() {
			continue
		}

		sessionValues := reflect.Indirect(fieldValue)

		if fieldValue.Kind() == reflect.Map {
			iter := sessionValues.MapRange()

			for iter.Next() {
				key := iter.Key()
				value := reflect.Indirect(iter.Value())

				builder.WriteString("\n[" + fmt.Sprintf("%v", key) + "]\n")

				for j := 0; j < value.NumField(); j++ {
					valueKey := value.Type().Field(j).Name
					valueVal := value.Field(j)

					if fmt.Sprintf("%v", valueVal) != "" {
						builder.WriteString(fmt.Sprintf("%v", toKebabCase(valueKey)) + "=" + fmt.Sprintf("%v", valueVal) + "\n")
					}
				}
			}
		} else {
			for j := 0; j < sessionValues.NumField(); j++ {
				sessionConfigName := sessionValues.Type().Field(j).Name
				sessionConfigValue := sessionValues.Field(j)

				if sessionConfigValue.String() != "" {

					if sessionConfigValue.Kind() == reflect.Slice {
						arrayString := sliceToString(sessionConfigValue, ",")
						builder.WriteString(toKebabCase(sessionConfigName) + "=" + fmt.Sprintf("%v", arrayString) + "\n")
					} else {
						builder.WriteString(toKebabCase(sessionConfigName) + "=" + fmt.Sprintf("%v", sessionConfigValue) + "\n")
					}
				}
			}
		}

		finalString := builder.String()
		// Ensure all config files have a trailing newline
		if !strings.HasSuffix(finalString, "\n") {
			finalString += "\n"
		}
		configMap[fieldTag] = finalString
	}
	return configMap
}

type WorkbenchSessionIniConfig struct {
	RSession     *WorkbenchRSessionConfig `json:"rsession.conf,omitempty"`
	Repos        *WorkbenchRepoConfig     `json:"repos.conf,omitempty"`
	WorkbenchNss *WorkbenchNssConfig      `json:"workbench_nss.conf,omitempty"`
	Positron     *WorkbenchPositronConfig `json:"positron.conf,omitempty"`
}

func (w *WorkbenchSessionIniConfig) GenerateConfigMap() map[string]string {
	configMap := make(map[string]string, countInitializedFields(*w))

	configStructValsPtr := reflect.ValueOf(w)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		var builder strings.Builder

		fieldName := configStructVals.Type().Field(i).Name
		field, _ := reflect.TypeOf(w).Elem().FieldByName(fieldName)
		fieldTag := string(field.Tag)
		fieldTag = strings.ReplaceAll(fieldTag, "json:\"", "")
		fieldTag = strings.ReplaceAll(fieldTag, ",omitempty\"", "")
		fieldValue := configStructVals.Field(i)

		if fieldValue.IsNil() {
			continue
		}

		sessionValues := reflect.Indirect(fieldValue)

		for j := 0; j < sessionValues.NumField(); j++ {
			sessionConfigName := sessionValues.Type().Field(j).Name
			sessionConfigValue := sessionValues.Field(j)

			if sessionConfigValue.String() != "" {

				if sessionConfigValue.Kind() == reflect.Slice {
					arrayString := sliceToString(sessionConfigValue, ",")
					builder.WriteString(toKebabCase(sessionConfigName) + "=" + fmt.Sprintf("%v", arrayString) + "\n")
				} else if sessionConfigName == "RSPM" || sessionConfigName == "CRAN" {
					builder.WriteString(sessionConfigName + "=" + fmt.Sprintf("%v", sessionConfigValue) + "\n")
				} else if sessionConfigName == "DefaultRSConnectServer" {
					builder.WriteString("default-rsconnect-server=" + fmt.Sprintf("%v", sessionConfigValue) + "\n")
				} else {
					builder.WriteString(toKebabCase(sessionConfigName) + "=" + fmt.Sprintf("%v", sessionConfigValue) + "\n")
				}
			}
		}
		finalString := builder.String()
		// Ensure all config files have a trailing newline
		if !strings.HasSuffix(finalString, "\n") {
			finalString += "\n"
		}
		configMap[fieldTag] = finalString
	}
	return configMap
}

type WorkbenchSessionNewlineConfig struct {
	VsCodeExtensionsConf   []string `json:"vscode.extensions.conf,omitempty"`
	PositronExtensionsConf []string `json:"positron.extensions.conf,omitempty"`
}

func (w *WorkbenchSessionNewlineConfig) GenerateConfigMap() map[string]string {
	configMap := make(map[string]string, countInitializedFields(*w))

	configStructValsPtr := reflect.ValueOf(w)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		var builder strings.Builder

		fieldName := configStructVals.Type().Field(i).Name
		field, _ := reflect.TypeOf(w).Elem().FieldByName(fieldName)
		fieldTag := string(field.Tag)
		fieldTag = strings.ReplaceAll(fieldTag, "json:\"", "")
		fieldTag = strings.ReplaceAll(fieldTag, ",omitempty\"", "")
		fieldValue := configStructVals.Field(i)

		if fieldValue.IsNil() {
			continue
		}

		sessionValue := reflect.Indirect(fieldValue)

		if sessionValue.String() != "" {
			if sessionValue.Kind() == reflect.Slice {
				arrayString := sliceToString(sessionValue, "\n")
				builder.WriteString(fmt.Sprintf("%v", arrayString))
			} else {
				builder.WriteString(fmt.Sprintf("%v", sessionValue) + "\n")
			}
		}
		finalString := builder.String()
		// Ensure all config files have a trailing newline
		if !strings.HasSuffix(finalString, "\n") {
			finalString += "\n"
		}
		configMap[fieldTag] = finalString
	}
	return configMap
}

type WorkbenchSessionJsonConfig struct {
	VSCodeUserSettingsJson   map[string]*apiextensionsv1.JSON `json:"vscode-user-settings.json,omitempty"`
	PositronUserSettingsJson map[string]*apiextensionsv1.JSON `json:"positron-user-settings.json,omitempty"`
}

func (w *WorkbenchSessionJsonConfig) GenerateConfigMap() map[string]string {
	configMap := make(map[string]string, countInitializedFields(*w))

	configStructValsPtr := reflect.ValueOf(w)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		fieldName := configStructVals.Type().Field(i).Name
		field, _ := reflect.TypeOf(w).Elem().FieldByName(fieldName)
		fieldTag := string(field.Tag)
		fieldTag = strings.ReplaceAll(fieldTag, "json:\"", "")
		fieldTag = strings.ReplaceAll(fieldTag, ",omitempty\"", "")
		fieldValue := configStructVals.Field(i)

		if fieldValue.IsNil() {
			continue
		}

		sessionValue := reflect.Indirect(fieldValue)

		if sessionValue.Kind() == reflect.Map {
			// Build a proper map for JSON marshaling
			jsonMap := make(map[string]interface{})
			mapIter := sessionValue.MapRange()

			for mapIter.Next() {
				key := mapIter.Key().String()
				value := mapIter.Value().Interface().(*apiextensionsv1.JSON).Raw

				var parsed interface{}
				if err := json.Unmarshal(value, &parsed); err == nil {
					jsonMap[key] = parsed
				}
			}

			// Marshal the map to proper JSON
			jsonBytes, err := json.Marshal(jsonMap)
			if err != nil {
				// Fallback to empty object if marshaling fails
				jsonBytes = []byte("{}")
			}

			finalString := string(jsonBytes)
			// Ensure all config files have a trailing newline
			if !strings.HasSuffix(finalString, "\n") {
				finalString += "\n"
			}
			configMap[fieldTag] = finalString
		}
	}
	return configMap
}

type WorkbenchDcfConfig struct {
	LauncherEnv *WorkbenchLauncherEnvConfig `json:"launcher-env,omitempty"`
}

// In the future we should make each config struct implement a common interface for this.
func (w *WorkbenchDcfConfig) GenerateConfigmap() map[string]string {
	configMap := make(map[string]string, countInitializedFields(*w))

	configStructValsPtr := reflect.ValueOf(w)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		var builder strings.Builder

		fieldName := configStructVals.Type().Field(i).Name
		field, _ := reflect.TypeOf(w).Elem().FieldByName(fieldName)
		fieldTag := string(field.Tag)
		fieldTag = strings.ReplaceAll(fieldTag, "json:\"", "")
		fieldTag = strings.ReplaceAll(fieldTag, ",omitempty\"", "")
		fieldValue := configStructVals.Field(i)

		if fieldValue.IsNil() {
			continue
		}

		dcfValues := reflect.Indirect(fieldValue)

		for j := 0; j < dcfValues.NumField(); j++ {
			childField := dcfValues.Type().Field(j)
			childFieldTag := string(childField.Tag)
			childFieldTag = strings.ReplaceAll(childFieldTag, "json:\"", "")
			childFieldTag = strings.ReplaceAll(childFieldTag, ",omitempty\"", "")
			childFieldValue := dcfValues.Field(j)

			if childFieldValue.Kind() == reflect.String {
				builder.WriteString(fmt.Sprintf("\n%s: %s", childFieldTag, childFieldValue))
			} else if childFieldValue.Kind() == reflect.Map {
				builder.WriteString(fmt.Sprintf("\n%s: ", childFieldTag))

				keys := make([]string, 0, childFieldValue.Len())
				mapIter := childFieldValue.MapRange()
				for mapIter.Next() {
					keys = append(keys, mapIter.Key().String())
				}
				sort.Strings(keys)

				first := true
				for _, key := range keys {
					value := childFieldValue.MapIndex(reflect.ValueOf(key)).String()

					if first {
						builder.WriteString(fmt.Sprintf("%s=%s\n", key, value))
						first = false
					} else {
						builder.WriteString(fmt.Sprintf("  %s=%s\n", key, value))
					}
				}
			}
		}

		finalString := builder.String()
		// Ensure all config files have a trailing newline
		if !strings.HasSuffix(finalString, "\n") {
			finalString += "\n"
		}
		configMap[fieldTag] = finalString
	}

	return configMap
}

type WorkbenchPositronConfig struct {
	Enabled                      int      `json:"enabled,omitempty"`
	Exe                          string   `json:"exe,omitempty"`
	Args                         string   `json:"args,omitempty"`
	DefaultSessionContainerImage string   `json:"default-session-container-image,omitempty"`
	SessionContainerImages       []string `json:"session-container-images,omitempty"`
	PositronSessionPath          string   `json:"positron-session-path,omitempty"`
	SessionNoProfile             int      `json:"session-no-profile,omitempty"`
	UserDataDir                  string   `json:"user-data-dir,omitempty"`
	AllowFileDownloads           int      `json:"allow-file-downloads,omitempty"`
	AllowFileUploads             int      `json:"allow-file-uploads,omitempty"`
	SessionTimeoutKillHours      int      `json:"session-timeout-kill-hours,omitempty"`
}

type WorkbenchRepoConfig struct {
	RSPM string `json:"RSPM,omitempty"`
	CRAN string `json:"CRAN,omitempty"`
}

type WorkbenchNssConfig struct {
	ServerAddress string `json:"server-address,omitempty"`
}

type WorkbenchRSessionConfig struct {
	DefaultRSConnectServer          string `json:"default-rsconnect-server,omitempty"`
	CopilotEnabled                  int    `json:"copilot-enabled,omitempty"`
	ManagedCredentialsInJobsEnabled int    `json:"managed-credentials-in-jobs-enabled,omitempty"`
	// SessionFirstProjectTemplatePath must be configured in the "project template," but _is_ a per-session-image setting
	SessionFirstProjectTemplatePath string            `json:"session-first-project-template-path,omitempty"`
	SessionSaveActionDefault        SessionSaveAction `json:"session-save-action-default,omitempty"`
}

type SessionSaveAction string

const (
	SessionSaveActionNone  SessionSaveAction = "no"
	SessionSaveActionAsk                     = "ask"
	SessionSaveActionYes                     = "yes"
	SessionSaveActionEmpty                   = ""
)

type WorkbenchLauncherKubnernetesResourcesConfigSection struct {
	Name                 string `json:"name,omitempty"`
	Cpus                 string `json:"cpus,omitempty"`
	CpusRequest          string `json:"cpus-request,omitempty"`
	MemMb                string `json:"mem-mb,omitempty"`
	MemMbRequest         string `json:"mem-mb-request,omitempty"`
	NvidiaGpus           string `json:"nvidia-gpus,omitempty"`
	AmdGpus              string `json:"amd-gpus,omitempty"`
	PlacementConstraints string `json:"placement-constraints,omitempty"`
}

type WorkbenchLauncherEnvConfig struct {
	JobType     string            `json:"JobType,omitempty"`
	Environment map[string]string `json:"Environment,omitempty"`
}

// Profiles //

type WorkbenchLauncherKubernetesProfilesConfigSection struct {
	ContainerImages       []string `json:"container-images,omitempty"`
	DefaultContainerImage string   `json:"default-container-image,omitempty"`
	AllowUnknownImages    int      `json:"allow-unknown-images,omitempty"`
	PlacementConstraints  []string `json:"placement-constraints,omitempty"`
	DefaultCpus           string   `json:"default-cpus,omitempty"`
	DefaultCpusRequest    string   `json:"default-cpus-request,omitempty"`
	DefaultMemMb          string   `json:"default-mem-mb,omitempty"`
	DefaultMemMbRequest   string   `json:"default-mem-mb-request,omitempty"`
	DefaultNvidiaGpus     string   `json:"default-nvidia-gpus,omitempty"`
	MaxCpus               string   `json:"max-cpus,omitempty"`
	MaxCpusRequest        string   `json:"max-cpus-request,omitempty"`
	MaxMemMb              string   `json:"max-mem-mb,omitempty"`
	MaxMemMbRequest       string   `json:"max-mem-mb-request,omitempty"`
	MaxNvidiaGpus         string   `json:"max-nvidia-gpus,omitempty"`
	CpuRequestRatio       string   `json:"cpu-request-ratio,omitempty"`
	MemoryRequestRatio    string   `json:"memory-request-ratio,omitempty"`
	ResourceProfiles      []string `json:"resource-profiles,omitempty"`
	AllowCustomResources  int      `json:"allow-custom-resources,omitempty"`
}

type WorkbenchProfilesConfig struct {
	LauncherKubernetesProfiles map[string]WorkbenchLauncherKubernetesProfilesConfigSection `json:"launcher.kubernetes.profiles.conf,omitempty"`
}

func (w *WorkbenchProfilesConfig) GenerateConfigMap() map[string]string {

	configMap := make(map[string]string, countInitializedFields(*w))

	configStructValsPtr := reflect.ValueOf(w)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		var builder strings.Builder

		fieldName := configStructVals.Type().Field(i).Name
		field, _ := reflect.TypeOf(w).Elem().FieldByName(fieldName)
		fieldTag := string(field.Tag)
		fieldTag = strings.ReplaceAll(fieldTag, "json:\"", "")
		fieldTag = strings.ReplaceAll(fieldTag, ",omitempty\"", "")
		fieldValue := configStructVals.Field(i)

		if fieldValue.IsNil() {
			continue
		}

		profiles := reflect.Indirect(fieldValue)

		iter := profiles.MapRange()

		for iter.Next() {
			profileName := iter.Key()
			profileValues := iter.Value()

			builder.WriteString("\n[" + fmt.Sprintf("%v", profileName) + "]\n")

			for j := 0; j < profileValues.NumField(); j++ {
				profileConfigName := profileValues.Type().Field(j).Name
				profileConfigValue := profileValues.Field(j)

				if profileConfigValue.String() != "" {
					if profileConfigValue.Kind() == reflect.Slice {
						arrayString := sliceToString(profileConfigValue, ",")
						if fmt.Sprintf("%v", arrayString) != "" {
							builder.WriteString(toKebabCase(profileConfigName) + "=" + fmt.Sprintf("%v", arrayString) + "\n")
						}
					} else if fmt.Sprintf("%v", profileConfigValue) != "" {
						builder.WriteString(toKebabCase(profileConfigName) + "=" + fmt.Sprintf("%v", profileConfigValue) + "\n")
					}
				}
			}
		}
		finalString := builder.String()
		// Ensure all config files have a trailing newline
		if !strings.HasSuffix(finalString, "\n") {
			finalString += "\n"
		}
		configMap[fieldTag] = finalString
	}

	return configMap
}

// Supervisord //

type SupervisordIniConfig struct {
	Programs map[string]map[string]*SupervisordProgramConfig `json:"programs.conf,omitempty"`
}

type SupervisordProgramConfig struct {
	User                  string `json:"user,omitempty"`
	Command               string `json:"command,omitempty"`
	AutoRestart           bool   `json:"autorestart,omitempty"`
	NumProcs              int    `json:"numprocs,omitempty"`
	StdOutLogFile         string `json:"stdout_logfile,omitempty"`
	StdOutLogFileMaxBytes int    `json:"stdout_logfile_maxbytes,omitempty"`
	StdErrLogFile         string `json:"stderr_logfile,omitempty"`
	StdErrLogFileMaxBytes int    `json:"stderr_logfile_maxbytes,omitempty"`
	Environment           string `json:"environment,omitempty"`
}

type WorkbenchIniConfig struct {
	Launcher           *WorkbenchLauncherConfig                                       `json:"launcher.conf,omitempty"`
	VsCode             *WorkbenchVsCodeConfig                                         `json:"vscode.conf,omitempty"`
	Logging            *WorkbenchLoggingConfig                                        `json:"logging.conf,omitempty"`
	Jupyter            *WorkbenchJupyterConfig                                        `json:"jupyter.conf,omitempty"`
	RServer            *WorkbenchRServerConfig                                        `json:"rserver.conf,omitempty"`
	LauncherKubernetes *WorkbenchLauncherKubernetesConfig                             `json:"launcher.kubernetes.conf,omitempty"`
	LauncherLocal      *WorkbenchLauncherLocalConfig                                  `json:"launcher.local.conf,omitempty"`
	Databricks         map[string]*WorkbenchDatabricksConfig                          `json:"databricks.conf,omitempty"` // TODO: DEPRECATED
	Resources          map[string]*WorkbenchLauncherKubnernetesResourcesConfigSection `json:"launcher.kubernetes.resources.conf,omitempty"`
}

type WorkbenchDatabricksConfig struct {
	Name         string `json:"name,omitempty"`
	Url          string `json:"url,omitempty"`
	ClientId     string `json:"client-id,omitempty"`
	ClientSecret string `json:"client-secret,omitempty"`
}

func (w *WorkbenchIniConfig) GenerateConfigMap() map[string]string {
	configMap := make(map[string]string, countInitializedFields(*w))

	configStructValsPtr := reflect.ValueOf(w)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		var builder strings.Builder

		fieldName := configStructVals.Type().Field(i).Name
		field, _ := reflect.TypeOf(w).Elem().FieldByName(fieldName)
		fieldTag := string(field.Tag)
		fieldTag = strings.ReplaceAll(fieldTag, "json:\"", "")
		fieldTag = strings.ReplaceAll(fieldTag, ",omitempty\"", "")
		fieldValue := configStructVals.Field(i)

		if fieldValue.IsNil() {
			continue
		}

		sectionStructVals := reflect.Indirect(fieldValue)

		if fieldValue.Kind() == reflect.Map {
			// Special handling for Resources field - sort by CPU/memory
			if fieldName == "Resources" {
				// Collect all resource profiles
				type resourceProfile struct {
					key   string
					value *WorkbenchLauncherKubnernetesResourcesConfigSection
				}
				var profiles []resourceProfile

				iter := sectionStructVals.MapRange()
				for iter.Next() {
					key := iter.Key().String()
					value, ok := iter.Value().Interface().(*WorkbenchLauncherKubnernetesResourcesConfigSection)
					if !ok {
						// Skip invalid entries
						continue
					}
					profiles = append(profiles, resourceProfile{key: key, value: value})
				}

				// Sort by CPU, then by memory (both low to high), with "default" always first
				sort.Slice(profiles, func(i, j int) bool {
					// "default" profile always comes first
					if profiles[i].key == "default" {
						return true
					}
					if profiles[j].key == "default" {
						return false
					}
					return compareResourceProfiles(profiles[i].value, profiles[j].value, profiles[i].key, profiles[j].key)
				})

				// Write sorted profiles
				for _, profile := range profiles {
					builder.WriteString("\n[" + profile.key + "]\n")
					value := reflect.ValueOf(profile.value).Elem()
					for j := 0; j < value.NumField(); j++ {
						valueKey := value.Type().Field(j).Name
						valueVal := value.Field(j)

						if fmt.Sprintf("%v", valueVal) != "" {
							builder.WriteString(fmt.Sprintf("%v", toKebabCase(valueKey)) + "=" + fmt.Sprintf("%v", valueVal) + "\n")
						}
					}
				}
			} else {
				// Default handling for other map fields
				iter := sectionStructVals.MapRange()

				for iter.Next() {
					key := iter.Key()
					value := reflect.Indirect(iter.Value())

					builder.WriteString("\n[" + fmt.Sprintf("%v", key) + "]\n")

					for j := 0; j < value.NumField(); j++ {
						valueKey := value.Type().Field(j).Name
						valueVal := value.Field(j)

						if fmt.Sprintf("%v", valueVal) != "" {
							builder.WriteString(fmt.Sprintf("%v", toKebabCase(valueKey)) + "=" + fmt.Sprintf("%v", valueVal) + "\n")
						}
					}
				}
			}
		} else {
			for j := 0; j < sectionStructVals.NumField(); j++ {
				sectionFieldName := sectionStructVals.Type().Field(j).Name
				sectionFieldValue := reflect.Indirect(sectionStructVals.Field(j))

				if sectionStructVals.Field(j).String() == "" {
					continue
				}

				if sectionFieldValue.Kind() == reflect.Struct {
					if toKebabCase(sectionFieldName) == "all" {
						builder.WriteString("\n[*]\n")
					} else {
						builder.WriteString("\n[" + toKebabCase(sectionFieldName) + "]\n")
					}

					for k := 0; k < sectionFieldValue.NumField(); k++ {
						parameter := sectionFieldValue.Type().Field(k).Name
						parameterValue := sectionFieldValue.Field(k)

						if fmt.Sprintf("%v", parameterValue) != "" {
							builder.WriteString(fmt.Sprintf("%v", toKebabCase(parameter)) + "=" + fmt.Sprintf("%v", parameterValue) + "\n")
						}
					}
				} else if sectionFieldValue.Kind() == reflect.Slice {
					// Special handling for auth-openid-scopes which requires space separation
					var separator string
					if toKebabCase(sectionFieldName) == "auth-openid-scopes" {
						separator = " "
					} else {
						separator = ","
					}
					arrayString := sliceToString(sectionFieldValue, separator)
					if arrayString != "" {
						builder.WriteString(fmt.Sprintf("%v", toKebabCase(sectionFieldName)) + "=" + arrayString + "\n")
					}
				} else if fmt.Sprintf("%v", sectionFieldValue) != "" && (sectionFieldValue.Kind() == reflect.String || sectionFieldValue.Kind() == reflect.Int) {
					builder.WriteString(fmt.Sprintf("%v", toKebabCase(sectionFieldName)) + "=" + fmt.Sprintf("%v", sectionFieldValue) + "\n")
				}
			}
		}

		finalString := builder.String()
		// Ensure all config files have a trailing newline
		if !strings.HasSuffix(finalString, "\n") {
			finalString += "\n"
		}
		configMap[fieldTag] = finalString
	}

	return configMap
}

// WorkbenchConfig is a "top-level" configuration object.
// It has "child-structs" which have different config formats, and the `GenerateConfigmap`
// method generates a map[string]string which can be used to create a configmap with the contents
type WorkbenchConfig struct {
	WorkbenchIniConfig            `json:"workbench-ini-config,omitempty"`
	WorkbenchSessionIniConfig     `json:"workbench-session-ini-config,omitempty"`
	WorkbenchSessionNewlineConfig `json:"workbench-session-newline-config,omitempty"`
	WorkbenchSessionJsonConfig    `json:"workbench-session-json-config,omitempty"`
	WorkbenchProfilesConfig       `json:"workbench-profiles-config,omitempty"`
	WorkbenchDcfConfig            `json:"workbench-dcf-config,omitempty"`
	// SupervisordIniConfig allows customization of the startup of the product... it is currently only enabled
	// and utilized when workbench.Spec.NonRoot is enabled
	SupervisordIniConfig `json:"supervisord-ini-config,omitempty"`
}

func (w *WorkbenchConfig) GenerateSessionConfigmap() (map[string]string, error) {
	m := map[string]string{}

	if countInitializedFields(w.WorkbenchSessionIniConfig) != 0 {
		for k, v := range w.WorkbenchSessionIniConfig.GenerateConfigMap() {
			m[k] = v
		}
	}

	if countInitializedFields(w.WorkbenchSessionNewlineConfig) != 0 {
		for k, v := range w.WorkbenchSessionNewlineConfig.GenerateConfigMap() {
			m[k] = v
		}
	}

	if countInitializedFields(w.WorkbenchSessionJsonConfig) != 0 {
		for k, v := range w.WorkbenchSessionJsonConfig.GenerateConfigMap() {
			m[k] = v
		}
	}

	return m, nil
}

// TODO: if we like this, we should pull this out into something we can use elsewhere too...
func getLoggerWithFallback(ctx context.Context, fallbackName string) logr.Logger {
	if l, err := logr.FromContext(ctx); err != nil {
		return ctrl.Log.WithName(fallbackName)
	} else {
		return l
	}
}

func (w *WorkbenchConfig) GenerateLoginConfigmapData(ctx context.Context) (map[string]string, error) {
	cmData := map[string]string{}

	for key, relPath := range map[string]string{
		"login.defs":     "_assets/etc/login.defs",
		"common-session": "_assets/etc/pam.d/common-session",
		"99-ptd.sh":      "_assets/etc/profile.d/99-ptd.sh",
	} {
		b, err := assets.ReadFile(relPath)
		if err != nil {
			return nil, err
		}

		cmData[key] = string(b)
	}

	return cmData, nil
}

func (w *WorkbenchConfig) GenerateSupervisorConfigmap(ctx context.Context) (map[string]string, error) {

	// To-Do: We need to count the number of maps within the first map,
	// instead of the programs.conf entries for this case specifically
	// For now this is fine since it's 1:1 but will cause issues in the
	// future if we don't fix it
	configMap := make(map[string]string, countInitializedFields(*w))

	configStructValsPtr := reflect.ValueOf(w.SupervisordIniConfig)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		var builder strings.Builder

		fieldValue := configStructVals.Field(i)

		if fieldValue.IsNil() {
			continue
		}

		programs := reflect.Indirect(fieldValue)

		iter := programs.MapRange()

		for iter.Next() {
			fileName := fmt.Sprintf("%v", iter.Key())
			programMap := iter.Value()

			programIter := programMap.MapRange()

			for programIter.Next() {
				programName := programIter.Key()
				programValues := reflect.Indirect(programIter.Value())

				builder.WriteString("[program:" + fmt.Sprintf("%v", programName) + "]\n")

				for j := 0; j < programValues.NumField(); j++ {
					programConfigName := programValues.Type().Field(j).Name
					programConfigValue := programValues.Field(j)

					programConfigName = insertAfter(programConfigName, "StdOut", "_")
					programConfigName = insertAfter(programConfigName, "StdErr", "_")
					programConfigName = insertAfter(programConfigName, "LogFile", "_")

					if programConfigValue.String() != "" {
						builder.WriteString(strings.ToLower(programConfigName) + "=" + fmt.Sprintf("%v", programConfigValue) + "\n")
					}
				}
			}
			configMap[fileName] = builder.String()
		}
	}
	return configMap, nil
}

func (w *WorkbenchConfig) GenerateConfigmap() (map[string]string, error) {
	// TIP: This `m` variable holds the file name as the key and the file content as the values
	// Helpful for re-factoring the functions below.
	m := map[string]string{}

	if countInitializedFields(w.WorkbenchIniConfig) != 0 {
		for k, v := range w.WorkbenchIniConfig.GenerateConfigMap() {
			m[k] = v
		}
	}

	if countInitializedFields(w.WorkbenchSessionIniConfig) != 0 {
		for k, v := range w.WorkbenchSessionIniConfig.GenerateConfigMap() {
			m[k] = v
		}
	}

	// Synchronize placement constraints from resources to profiles before generating profiles config
	w.syncPlacementConstraints()

	if countInitializedFields(w.WorkbenchProfilesConfig) != 0 {
		for k, v := range w.WorkbenchProfilesConfig.GenerateConfigMap() {
			m[k] = v
		}
	}

	if countInitializedFields(w.WorkbenchDcfConfig) != 0 {
		for k, v := range w.WorkbenchDcfConfig.GenerateConfigmap() {
			m[k] = v
		}
	}

	if countInitializedFields(w.WorkbenchSessionJsonConfig) != 0 {
		for k, v := range w.WorkbenchSessionJsonConfig.GenerateConfigMap() {
			m[k] = v
		}
	}

	if countInitializedFields(w.WorkbenchSessionNewlineConfig) != 0 {
		for k, v := range w.WorkbenchSessionNewlineConfig.GenerateConfigMap() {
			m[k] = v
		}
	}

	return m, nil
}

func (w *WorkbenchSecretConfig) GenerateSecretData() (map[string]string, error) {
	m := map[string]string{}
	if countInitializedFields(w.WorkbenchSecretIniConfig) != 0 {
		for k, v := range w.WorkbenchSecretIniConfig.GenerateConfigMap() {
			m[k] = v
		}
	}
	return m, nil
}

type WorkbenchDatabaseProvider string

const (
	WorkbenchDatabaseProviderPostgres WorkbenchDatabaseProvider = "postgresql"
	WorkbenchDatabaseProviderSqlite   WorkbenchDatabaseProvider = "sqlite"
)

type WorkbenchOpenidClientSecret struct {
	ClientId     string `json:"client-id,omitempty"`
	ClientSecret string `json:"client-secret,omitempty"`
}

type WorkbenchDatabaseConfig struct {
	Provider WorkbenchDatabaseProvider `json:"provider,omitempty"`
	Database string                    `json:"database,omitempty"`
	Port     string                    `json:"port,omitempty"`
	Host     string                    `json:"host,omitempty"`
	Username string                    `json:"username,omitempty"`
	Password string                    `json:"password,omitempty"`
}

type WorkbenchVsCodeConfig struct {
	Enabled                 int    `json:"enabled,omitempty"`
	Exe                     string `json:"exe,omitempty"`
	Args                    string `json:"args,omitempty"`
	SessionTimeoutKillHours int    `json:"session-timeout-kill-hours,omitempty"`
}

type WorkbenchJupyterConfig struct {
	// NotebooksEnabled enables Jupyter Notebook Classic sessions
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	// +kubebuilder:default=0
	NotebooksEnabled int `json:"notebooks-enabled,omitempty"`

	// LabsEnabled enables JupyterLab sessions
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	// +kubebuilder:default=1
	LabsEnabled int `json:"labs-enabled,omitempty"`

	// JupyterExe specifies the path to the Jupyter executable
	// +kubebuilder:default=""
	JupyterExe string `json:"jupyter-exe,omitempty"`

	// LabVersion specifies the JupyterLab version to use
	// +kubebuilder:default="auto"
	LabVersion string `json:"lab-version,omitempty"`

	// NotebookVersion specifies the Jupyter Notebook version to use
	// +kubebuilder:default="auto"
	NotebookVersion string `json:"notebook-version,omitempty"`

	// SessionCullMinutes specifies idle time in minutes before kernels/terminals are culled
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=120
	SessionCullMinutes int `json:"session-cull-minutes,omitempty"`

	// SessionShutdownMinutes specifies idle time in minutes before session shutdown
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=5
	SessionShutdownMinutes int `json:"session-shutdown-minutes,omitempty"`

	// DefaultSessionCluster specifies the default Job Launcher cluster for Jupyter sessions
	DefaultSessionCluster string `json:"default-session-cluster,omitempty"`

	// DefaultSessionContainerImage specifies the default container image for Jupyter sessions
	DefaultSessionContainerImage string `json:"default-session-container-image,omitempty"`
}

type WorkbenchLoggingConfig struct {
	All *WorkbenchLoggingSection `json:"*,omitempty"`
}

type WorkbenchLogLevel string

const (
	WorkbenchLogLevelInfo  WorkbenchLogLevel = "info"
	WorkbenchLogLevelDebug                   = "debug"
	WorkbenchLogLevelWarn                    = "warn"
	WorkbenchLogLevelError                   = "error"
)

type WorkbenchLogFormat string

const (
	WorkbenchLogFormatPretty WorkbenchLogFormat = "pretty"
	WorkbenchLogFormatJson                      = "json"
)

type WorkbenchLoggerType string

const (
	WorkbenchLoggerTypeStdErr WorkbenchLoggerType = "stderr"
	WorkbenchLoggerTypeSysLog                     = "syslog"
	WorkbenchLoggerTypeFile                       = "file"
)

type WorkbenchLoggingSection struct {
	LogLevel         WorkbenchLogLevel   `json:"log-level,omitempty"`
	LoggerType       WorkbenchLoggerType `json:"logger-type,omitempty"`
	LogMessageFormat WorkbenchLogFormat  `json:"log-message-format,omitempty"`
}

type WorkbenchRServerConfig struct {
	//+kubebuilder:default=1
	LoadBalancingEnabled                   int      `json:"load-balancing-enabled,omitempty"`
	ServerSharedStoragePath                string   `json:"server-shared-storage-path,omitempty"`
	ServerHealthCheckEnabled               int      `json:"server-health-check-enabled,omitempty"`
	AuthPamSessionsEnabled                 int      `json:"auth-pam-sessions-enabled,omitempty"`
	AdminEnabled                           int      `json:"admin-enabled,omitempty"`
	AdminGroup                             string   `json:"admin-group,omitempty"`
	AdminSuperuserGroup                    string   `json:"admin-superuser-group,omitempty"`
	WwwPort                                int      `json:"www-port,omitempty"`
	ServerProjectSharing                   int      `json:"server-project-sharing,omitempty"`
	LauncherAddress                        string   `json:"launcher-address,omitempty"`
	LauncherPort                           int      `json:"launcher-port,omitempty"`
	LauncherSessionsEnabled                int      `json:"launcher-sessions-enabled,omitempty"`
	LauncherSessionsCallbackAddress        string   `json:"launcher-sessions-callback-address,omitempty"`
	AuthOpenid                             int      `json:"auth-openid,omitempty"`
	AuthOpenidIssuer                       string   `json:"auth-openid-issuer,omitempty"`
	AuthOpenidUsernameClaim                string   `json:"auth-openid-username-claim,omitempty"`
	AuthOpenidScopes                       []string `json:"auth-openid-scopes,omitempty"`
	AuthSaml                               int      `json:"auth-saml,omitempty"`
	AuthSamlMetadataUrl                    string   `json:"auth-saml-metadata-url,omitempty"`
	AuthSamlSpAttributeUsername            string   `json:"auth-saml-sp-attribute-username,omitempty"`
	WwwFrameOrigin                         string   `json:"www-frame-origin,omitempty"`
	UserProvisioningEnabled                int      `json:"user-provisioning-enabled,omitempty"`
	UserProvisioningRegisterOnFirstLogin   int      `json:"user-provisioning-register-on-first-login,omitempty"`
	UserHomedirPath                        string   `json:"user-homedir-path,omitempty"`
	SecureCookieKeyFile                    string   `json:"secure-cookie-key-file,omitempty"`
	MetricsEnabled                         int      `json:"metrics-enabled,omitempty"`
	MetricsPort                            int      `json:"metrics-port,omitempty"`
	WwwThreadPoolSize                      int      `json:"www-thread-pool-size,omitempty"`
	LauncherSessionsProxyTimeoutSeconds    int      `json:"launcher-sessions-proxy-timeout-seconds,omitempty"`
	LauncherSessionsAutoUpdate             int      `json:"launcher-sessions-auto-update,omitempty"`
	LauncherSessionsInitContainerImageName string   `json:"launcher-sessions-init-container-image-name,omitempty"`
	LauncherSessionsInitContainerImageTag  string   `json:"launcher-sessions-init-container-image-tag,omitempty"`
	DatabricksEnabled                      int      `json:"databricks-enabled,omitempty"`
	WorkbenchApiEnabled                    int      `json:"workbench-api-enabled,omitempty"`
	WorkbenchApiAdminEnabled               int      `json:"workbench-api-admin-enabled,omitempty"`
	WorkbenchApiSuperAdminEnabled          int      `json:"workbench-api-super-admin-enabled,omitempty"`
	ForceAdminUiEnabled                    int      `json:"force-admin-ui-enabled,omitempty"`
}

type WorkbenchLauncherConfig struct {
	Server  *WorkbenchLauncherServerConfig  `json:"server,omitempty"`
	Cluster *WorkbenchLauncherClusterConfig `json:"cluster,omitempty"`
}

type WorkbenchLauncherKubernetesConfig struct {
	KubernetesNamespace string `json:"kubernetes-namespace,omitempty"`
	UseTemplating       int    `json:"use-templating,omitempty"`
	JobExpiryHours      int    `json:"job-expiry-hours,omitempty"`
}

type WorkbenchLauncherLocalConfig struct {
	Unprivileged int `json:"unprivileged,omitempty"`
}

type WorkbenchLauncherClusterConfig struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

type WorkbenchLauncherServerConfig struct {
	Address              string `json:"address,omitempty"`
	Port                 string `json:"port,omitempty"`
	ServerUser           string `json:"server-user,omitempty"`
	AdminGroup           string `json:"admin-group,omitempty"`
	AuthorizationEnabled int    `json:"authorization-enabled,omitempty"`
	ThreadPoolSize       int    `json:"thread-pool-size,omitempty"`
	Unprivileged         int    `json:"unprivileged,omitempty"`
	SecureCookieKeyFile  string `json:"secure-cookie-key-file,omitempty"`
}

func sliceToString(sliceValue reflect.Value, separator string) string {
	var arrayString string
	for k := 0; k < sliceValue.Len(); k++ {
		arrayValue := sliceValue.Index(k).String()
		if arrayValue != "" {
			if k == 0 {
				arrayString += arrayValue
			} else {
				arrayString += separator + arrayValue
			}
		}
	}
	return arrayString
}

func toKebabCase(input string) string {
	var result strings.Builder

	for i, r := range input {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('-')
		}
		result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

func countInitializedFields(s any) int {
	// Ensure the input is a struct
	val := reflect.ValueOf(s)
	if val.Kind() != reflect.Struct {
		panic("Input must be a struct" + fmt.Sprintf("%v", val.Kind()))
	}

	initializedCount := 0
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)

		// Check if the field is initialized (non-zero)
		if !field.IsZero() {
			initializedCount++
		}
	}

	return initializedCount
}

func insertAfter(input, target, insert string) string {
	// Find the index of the target substring
	pos := strings.Index(input, target)
	if pos == -1 {
		// Return the original string if the target is not found
		return input
	}

	// Construct the new string
	return input[:pos+len(target)] + insert + input[pos+len(target):]
}

// parseResourceQuantity parses a resource value string and returns a Quantity.
// Returns a zero quantity if the value is empty or invalid.
func parseResourceQuantity(value string) resource.Quantity {
	if value == "" {
		return resource.MustParse("0")
	}
	q, err := resource.ParseQuantity(value)
	if err != nil {
		return resource.MustParse("0")
	}
	return q
}

// getEffectiveResource returns the maximum of limit and request values for a resource
func getEffectiveResource(limit, request string) resource.Quantity {
	limitQ := parseResourceQuantity(limit)
	requestQ := parseResourceQuantity(request)
	if limitQ.Cmp(requestQ) > 0 {
		return limitQ
	}
	return requestQ
}

// compareResourceProfiles compares two resource profiles for sorting.
// Returns true if profile i should come before profile j.
// Sorting is done by CPU first (using the higher of Cpus or CpusRequest),
// then by memory (using the higher of MemMb or MemMbRequest).
func compareResourceProfiles(i, j *WorkbenchLauncherKubnernetesResourcesConfigSection, iKey, jKey string) bool {
	// Get effective CPU values
	iEffectiveCpu := getEffectiveResource(i.Cpus, i.CpusRequest)
	jEffectiveCpu := getEffectiveResource(j.Cpus, j.CpusRequest)

	// Compare by CPU first
	cpuCmp := iEffectiveCpu.Cmp(jEffectiveCpu)
	if cpuCmp != 0 {
		return cpuCmp < 0
	}

	// If CPU is equal, compare by memory
	iEffectiveMem := getEffectiveResource(i.MemMb, i.MemMbRequest)
	jEffectiveMem := getEffectiveResource(j.MemMb, j.MemMbRequest)

	memCmp := iEffectiveMem.Cmp(jEffectiveMem)
	if memCmp != 0 {
		return memCmp < 0
	}

	// If both CPU and memory are equal, sort by profile key for consistency
	return iKey < jKey
}

// parseConstraintsString parses a comma-separated string of placement constraints
// into a slice of individual constraint strings
func parseConstraintsString(constraints string) []string {
	if strings.TrimSpace(constraints) == "" {
		return []string{}
	}

	parts := strings.Split(constraints, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// mergeConstraints merges two slices of constraints and removes duplicates
// The existing constraints are preserved first, then new ones are added
func mergeConstraints(existing, new []string) []string {
	if existing == nil {
		existing = []string{}
	}
	if new == nil {
		new = []string{}
	}

	// Use a map to track unique constraints
	seen := make(map[string]bool)
	result := make([]string, 0, len(existing)+len(new))

	// Add existing constraints first
	for _, c := range existing {
		if !seen[c] {
			seen[c] = true
			result = append(result, c)
		}
	}

	// Add new constraints
	for _, c := range new {
		if !seen[c] {
			seen[c] = true
			result = append(result, c)
		}
	}

	return result
}

// validateConstraintFormat validates that a constraint follows the key:value or key=value format
// with no spaces and non-empty key and value
func validateConstraintFormat(constraint string) bool {
	if constraint == "" {
		return false
	}

	// Support both colon (:) and equals (=) as separators
	// Colon is used in launcher config, equals is used in Kubernetes nodeSelector
	var parts []string
	if strings.Contains(constraint, ":") {
		parts = strings.Split(constraint, ":")
	} else if strings.Contains(constraint, "=") {
		parts = strings.Split(constraint, "=")
	} else {
		return false
	}

	if len(parts) != 2 {
		return false
	}

	key := parts[0]
	value := parts[1]

	// Check for empty key or value
	if key == "" || value == "" {
		return false
	}

	// Check for spaces in key or value
	if strings.Contains(key, " ") || strings.Contains(value, " ") {
		return false
	}

	return true
}

// syncPlacementConstraints synchronizes placement constraints from resource profiles
// to the launcher profiles that reference them
func (w *WorkbenchConfig) syncPlacementConstraints() {
	if w.Resources == nil || w.LauncherKubernetesProfiles == nil {
		return
	}

	logger := ctrl.Log.WithName("workbench-config")

	// Iterate through each launcher profile
	for profileName, profile := range w.LauncherKubernetesProfiles {
		if profile.ResourceProfiles == nil || len(profile.ResourceProfiles) == 0 {
			continue
		}

		// Collect constraints from referenced resource profiles
		var newConstraints []string
		for _, resourceName := range profile.ResourceProfiles {
			resource, exists := w.Resources[resourceName]
			if !exists {
				logger.Info("Resource profile not found", "profile", profileName, "resource", resourceName)
				continue
			}

			if resource.PlacementConstraints != "" {
				constraints := parseConstraintsString(resource.PlacementConstraints)
				for _, c := range constraints {
					if validateConstraintFormat(c) {
						newConstraints = append(newConstraints, c)
					} else {
						logger.Info("Invalid constraint format", "constraint", c, "resource", resourceName)
					}
				}
			}
		}

		// Merge with existing constraints
		profile.PlacementConstraints = mergeConstraints(profile.PlacementConstraints, newConstraints)
		w.LauncherKubernetesProfiles[profileName] = profile
	}
}
