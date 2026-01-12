package v1beta1

import (
	"fmt"

	"reflect"
	"strings"
)

type ChronicleConfig struct {
	Http         *ChronicleHttpConfig         `json:"Http,omitempty"`
	Metrics      *ChronicleMetricsConfig      `json:"Metrics,omitempty"`
	Profiling    *ChronicleProfilingConfig    `json:"Profiling,omitempty"`
	S3Storage    *ChronicleS3StorageConfig    `json:"S3Storage,omitempty"`
	LocalStorage *ChronicleLocalStorageConfig `json:"LocalStorage,omitempty"`
	Logging      *ChronicleLoggingConfig      `json:"Logging,omitempty"`
}

type ChronicleServiceLogLevel string

const (
	ChronicleServiceLogLevelInfo  ChronicleServiceLogLevel = "INFO"
	ChronicleServiceLogLevelDebug                          = "DEBUG"
)

type ChronicleServiceLogFormat string

const (
	ChronicleServiceLogFormatText ChronicleServiceLogFormat = "TEXT"
	ChronicleServiceLogFormatJson                           = "JSON"
)

type ChronicleLoggingConfig struct {
	ServiceLog       string                    `json:"ServiceLog,omitempty"`
	ServiceLogLevel  ChronicleServiceLogLevel  `json:"ServiceLogLevel,omitempty"`
	ServiceLogFormat ChronicleServiceLogFormat `json:"ServiceLogFormat,omitempty"`
}

type ChronicleProfilingConfig struct {
	Enabled bool   `json:"Enabled,omitempty"`
	Listen  string `json:"Listen,omitempty"`
}

type ChronicleHttpConfig struct {
	Listen string `json:"Listen,omitempty"`
}

type ChronicleMetricsConfig struct {
	Enabled bool   `json:"Enabled,omitempty"`
	Listen  string `json:"Listen,omitempty"`
}

type ChronicleS3StorageConfig struct {
	Enabled bool   `json:"Enabled,omitempty"`
	Bucket  string `json:"Bucket,omitempty"`
	Region  string `json:"Region,omitempty"`
	Prefix  string `json:"Prefix,omitempty"`
}

type ChronicleLocalStorageConfig struct {
	Enabled  bool   `json:"Enabled,omitempty"`
	Location string `json:"Location,omitempty"`
}

func (configStruct *ChronicleConfig) GenerateGcfg() (string, error) {
	var builder strings.Builder

	configStructValsPtr := reflect.ValueOf(configStruct)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		fieldName := configStructVals.Type().Field(i).Name
		fieldValue := configStructVals.Field(i)

		if fieldValue.IsNil() {
			continue
		}

		sectionStructVals := reflect.Indirect(fieldValue)

		builder.WriteString("\n[" + fieldName + "]\n")

		for j := 0; j < sectionStructVals.NumField(); j++ {
			sectionFieldName := sectionStructVals.Type().Field(j).Name
			sectionFieldValue := sectionStructVals.Field(j)

			if sectionStructVals.Field(j).String() != "" {
				if sectionFieldValue.Kind() == reflect.Slice {
					for k := 0; k < sectionFieldValue.Len(); k++ {
						arrayValue := sectionFieldValue.Index(k).String()
						if arrayValue != "" {
							builder.WriteString(fmt.Sprintf("%v", sectionFieldName) + " = " + fmt.Sprintf("%v", arrayValue) + "\n")
						}
					}

				} else {
					builder.WriteString(fmt.Sprintf("%v", sectionFieldName) + " = " + fmt.Sprintf("%v", sectionFieldValue) + "\n")
				}
			}
		}

	}
	return builder.String(), nil
}
