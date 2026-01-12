package product

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectDefaultRuntimeDefinition_BuildDefaultRuntimeYAML(t *testing.T) {
	fullImg := &ConnectRuntimeImageDefinition{
		PyVersion:     "3.8.16",
		RVersion:      "3.6.3",
		OSVersion:     "ubuntu2204",
		QuartoVersion: "1.3.340",
		Repo:          "ghcr.io/rstudio/content-pro",
	}

	overrideImage := ConnectRuntimeImageDefinition{
		Repo:        "repo",
		TagOverride: "some-tag",
		RVersion:    "1.2.3",
	}

	t.Run("typical", func(t *testing.T) {
		r := require.New(t)

		full := &ConnectRuntimeDefinition{
			Images: []RuntimeYAMLImageEntry{fullImg.MustGenerateImageEntry()},
		}

		str, err := full.BuildDefaultRuntimeYAML()
		r.NoError(err)
		r.Contains(str, "name: Kubernetes")
		r.Contains(str, "name: ghcr.io/rstudio/content-pro:r3.6.3-py3.8.16-ubuntu2204")
		r.Contains(str, "python:")
	})

	t.Run("with override", func(t *testing.T) {
		r := require.New(t)

		override := ConnectRuntimeDefinition{
			Images: []RuntimeYAMLImageEntry{
				overrideImage.MustGenerateImageEntry(),
			},
		}

		str, err := override.BuildDefaultRuntimeYAML()
		r.NoError(err)
		r.Contains(str, "name: Kubernetes")
		r.Contains(str, "name: repo:some-tag")
		r.NotContains(str, "python:")
		r.Contains(str, "path: /opt/R/1.2.3/bin/R")
	})

	t.Run("multiple", func(t *testing.T) {
		r := require.New(t)

		double := ConnectRuntimeDefinition{
			Images: []RuntimeYAMLImageEntry{
				fullImg.MustGenerateImageEntry(),
				overrideImage.MustGenerateImageEntry(),
			},
		}

		str, err := double.BuildDefaultRuntimeYAML()
		r.NoError(err)
		r.Contains(str, "name: Kubernetes")
		r.Contains(str, "name: ghcr.io/rstudio/content-pro:r3.6.3-py3.8.16-ubuntu2204")
		r.Contains(str, "python:")
		r.Contains(str, "path: /opt/R/3.6.3/bin/R")
		r.Contains(str, "path: /opt/R/1.2.3/bin/R")
	})
}

func TestConnectRuntimeImageDefinition(t *testing.T) {
	t.Run("GenerateImageEntry", func(t *testing.T) {
		t.Run("valid", func(t *testing.T) {
			r := require.New(t)
			def := &ConnectRuntimeImageDefinition{
				PyVersion:     "3.8.16",
				RVersion:      "3.6.3",
				OSVersion:     "ubuntu2204",
				QuartoVersion: "1.3.340",
				Repo:          "ghcr.io/rstudio/content-pro",
			}

			img, err := def.GenerateImageEntry()
			r.NoError(err)
			r.NotNil(img)
		})

		t.Run("missing python", func(t *testing.T) {
			r := require.New(t)
			def := &ConnectRuntimeImageDefinition{
				RVersion:      "3.6.3",
				OSVersion:     "ubuntu2204",
				QuartoVersion: "1.3.340",
				Repo:          "ghcr.io/rstudio/content-pro",
			}
			_, err := def.GenerateImageEntry()
			r.Error(err)
			r.ErrorIs(err, InvalidConnectRuntimeImageDefinitionError)
		})

		t.Run("missing r", func(t *testing.T) {
			r := require.New(t)
			def := &ConnectRuntimeImageDefinition{
				PyVersion:     "3.8.16",
				OSVersion:     "ubuntu2204",
				QuartoVersion: "1.3.340",
				Repo:          "ghcr.io/rstudio/content-pro",
			}
			_, err := def.GenerateImageEntry()
			r.Error(err)
			r.ErrorIs(err, InvalidConnectRuntimeImageDefinitionError)
		})

		t.Run("missing os", func(t *testing.T) {
			r := require.New(t)
			def := &ConnectRuntimeImageDefinition{
				PyVersion:     "3.8.16",
				RVersion:      "3.6.3",
				QuartoVersion: "1.3.340",
				Repo:          "ghcr.io/rstudio/content-pro",
			}
			_, err := def.GenerateImageEntry()
			r.Error(err)
			r.ErrorIs(err, InvalidConnectRuntimeImageDefinitionError)
		})
	})
}
