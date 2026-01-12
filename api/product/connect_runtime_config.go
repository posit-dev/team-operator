package product

import (
	"fmt"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var InvalidConnectRuntimeImageDefinitionError = errors.New("invalid connect runtime image definition")

type RuntimeYAML struct {
	Name   string                  `yaml:"name"`
	Images []RuntimeYAMLImageEntry `yaml:"images"`
}

type RuntimeYAMLImageEntry struct {
	Name   string             `yaml:"name"`
	Python RuntimeYAMLSection `yaml:"python,omitempty"`
	R      RuntimeYAMLSection `yaml:"r,omitempty"`
	Quarto RuntimeYAMLSection `yaml:"quarto,omitempty"`
}

type RuntimeYAMLSection struct {
	Installations []RuntimeYAMLInstallation `yaml:"installations,omitempty"`
}

type RuntimeYAMLInstallation struct {
	Path    string `yaml:"path"`
	Version string `yaml:"version"`
}

type ConnectRuntimeDefinition struct {
	Images []RuntimeYAMLImageEntry `yaml:"images"`
}

func (d *ConnectRuntimeDefinition) BuildDefaultRuntimeYAML() (string, error) {
	rObj := RuntimeYAML{
		Name:   "Kubernetes",
		Images: d.Images,
	}
	if yamlBytes, err := yaml.Marshal(rObj); err != nil {
		return "", err
	} else {
		return string(yamlBytes), nil
	}
}

type ConnectRuntimeImageDefinition struct {
	PyVersion     string
	RVersion      string
	OSVersion     string
	QuartoVersion string
	Repo          string
	TagOverride   string
}

func (r *ConnectRuntimeImageDefinition) MustGenerateImageEntry() RuntimeYAMLImageEntry {
	if ent, err := r.GenerateImageEntry(); err != nil {
		panic(err)
	} else {
		return ent
	}
}

func (r *ConnectRuntimeImageDefinition) GenerateImageEntry() (RuntimeYAMLImageEntry, error) {
	tag := r.TagOverride
	if tag == "" {
		if r.PyVersion == "" {
			return RuntimeYAMLImageEntry{}, errors.Wrap(InvalidConnectRuntimeImageDefinitionError, "missing python version defined")
		}
		if r.RVersion == "" {
			return RuntimeYAMLImageEntry{}, errors.Wrap(InvalidConnectRuntimeImageDefinitionError, "missing R version defined")
		}
		if r.OSVersion == "" {
			return RuntimeYAMLImageEntry{}, errors.Wrap(InvalidConnectRuntimeImageDefinitionError, "missing OS version defined")
		}
		tag = fmt.Sprintf("r%s-py%s-%s", r.RVersion, r.PyVersion, r.OSVersion)
	}

	optionalR := RuntimeYAMLSection{}
	if r.RVersion != "" {
		optionalR = RuntimeYAMLSection{Installations: []RuntimeYAMLInstallation{
			{
				Path:    fmt.Sprintf("/opt/R/%s/bin/R", r.RVersion),
				Version: r.RVersion,
			},
		}}
	}

	optionalPython := RuntimeYAMLSection{}
	if r.PyVersion != "" {
		optionalPython = RuntimeYAMLSection{Installations: []RuntimeYAMLInstallation{
			{
				Path:    fmt.Sprintf("/opt/python/%s/bin/python3", r.PyVersion),
				Version: r.PyVersion,
			},
		}}
	}

	optionalQuarto := RuntimeYAMLSection{}
	if r.QuartoVersion != "" {
		optionalQuarto = RuntimeYAMLSection{Installations: []RuntimeYAMLInstallation{
			{
				Path:    fmt.Sprintf("/opt/quarto/%s/bin/quarto", r.QuartoVersion),
				Version: r.QuartoVersion,
			},
		},
		}
	}

	return RuntimeYAMLImageEntry{
		Name:   fmt.Sprintf("%s:%s", r.Repo, tag),
		Python: optionalPython,
		R:      optionalR,
		Quarto: optionalQuarto,
	}, nil
}
