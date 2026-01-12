package templates

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	"github.com/posit-dev/team-operator/api/templates/contrib"
)

//go:embed _config.tpl
var configTpl string

//go:embed _profiles.tpl
var profilesTpl string

//go:embed _debug.tpl
var debugTpl string

//go:embed _launcher-templates.tpl
var launcherTemplatesTpl string

// TODO: we will probably need to support more than one version eventually...
//   (i.e. if Launcher version >= XYZ, then use this template, else older)

//go:embed 2.5.0/job.tpl
var jobTpl string

//go:embed 2.5.0/service.tpl
var serviceTpl string

func DumpJobTpl() string {
	return jobTpl
}

func DumpServiceTpl() string {
	return serviceTpl
}

const recursionMaxNums = 1000

func AddOnFuncMap(t *template.Template, f template.FuncMap) template.FuncMap {
	includedNames := map[string]int{}

	// Add the 'include' function here so we can close over t.
	f["include"] = func(name string, data interface{}) (string, error) {
		var buf strings.Builder
		if v, ok := includedNames[name]; ok {
			if v > recursionMaxNums {
				return "", errors.Wrapf(fmt.Errorf("unable to execute template"), "rendering template has a nested reference name: %s", name)
			}
			includedNames[name]++
		} else {
			includedNames[name] = 1
		}
		err := t.ExecuteTemplate(&buf, name, data)
		includedNames[name]--
		return buf.String(), err
	}

	return f
}

// TemplateFuncMap returns template functions that helm uses
func TemplateFuncMap(t *template.Template) template.FuncMap {
	// template functions that helm uses
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	// extra funcs pulled from helm
	extra := contrib.ExtraFuncMap()

	for k, v := range extra {
		f[k] = v
	}

	return f
}

func RenderTemplateDataOutput(mapData map[string]any) (string, error) {
	t := template.New("gotpl")

	// template functions that helm uses
	f := TemplateFuncMap(t)
	t.Funcs(f)

	// filepath from project root
	t, err := t.Parse(launcherTemplatesTpl)
	if err != nil {
		return "", err
	}
	t1 := t.Lookup("rstudio-library.templates.dataOutput")

	// execute template
	s := ""
	buf := bytes.NewBufferString(s)
	if err := t1.Execute(buf, mapData); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func GenerateIni(mapData any) (string, error) {
	t := template.New("gotpl-ini")

	f := TemplateFuncMap(t)
	t.Funcs(f)

	t, err := t.Parse(configTpl)
	if err != nil {
		return "", err
	}
	t1 := t.Lookup("rstudio-library.config.ini-entry")

	// execute template
	s := ""
	buf := bytes.NewBufferString(s)
	if err := t1.Execute(buf, mapData); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func GenerateJson(mapData any) (string, error) {
	t := template.New("gotpl-ini")

	f := TemplateFuncMap(t)
	t.Funcs(f)

	t, err := t.Parse(configTpl)
	if err != nil {
		return "", err
	}
	t1 := t.Lookup("rstudio-library.config.json-entry")

	// execute template
	s := ""
	buf := bytes.NewBufferString(s)
	if err := t1.Execute(buf, mapData); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func GenerateNewlines(mapData any) (string, error) {
	t := template.New("gotpl-ini")

	f := TemplateFuncMap(t)
	t.Funcs(f)

	t, err := t.Parse(configTpl)
	if err != nil {
		return "", err
	}
	t1 := t.Lookup("rstudio-library.config.newline-entry")

	// execute template
	s := ""
	buf := bytes.NewBufferString(s)
	if err := t1.Execute(buf, mapData); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var leadingSpaceReplace = regexp.MustCompile("\\n  ")

func GenerateIniProfiles(mapData any) (string, error) {
	t := template.New("gotpl-profiles-ini")

	f := TemplateFuncMap(t)

	f2 := AddOnFuncMap(t, f)
	t.Funcs(f2)

	// concatenate files together that have our dependencies...
	t, err := t.Parse(profilesTpl + "\n\n" + configTpl + "\n\n" + debugTpl)
	if err != nil {
		return "", err
	}

	//t1 := t.Lookup("rstudio-library.profiles.ini.advanced")
	t1 := t.Lookup("rstudio-library.profiles.apply-everyone-and-default-to-others")
	//t1 := t.Lookup("testing")
	if err != nil {
		return "", err
	}

	tmp := map[string]any{
		"data": mapData,
		// these are not used in our implementation...
		"jobJsonDefaults": []map[string]string{},
		"filePath":        "/tmp",
	}

	// execute template
	s := ""
	buf := bytes.NewBufferString(s)
	if err := t1.Execute(buf, tmp); err != nil {
		return "", err
	}

	// remove leading two spaces for consistency with other config outputs...
	var tmpStr = buf.String()
	tmpStr = leadingSpaceReplace.ReplaceAllString(tmpStr, "\n")

	return tmpStr, nil
}

func GenerateGcfg(mapData map[string]any) (string, error) {
	t := template.New("gotpl")

	f := TemplateFuncMap(t)
	t.Funcs(f)

	t, err := t.Parse(configTpl)
	if err != nil {
		return "", err
	}
	t1 := t.Lookup("rstudio-library.config.gcfg")

	// execute template
	s := ""
	buf := bytes.NewBufferString(s)
	if err := t1.Execute(buf, mapData); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ConvertStructToMapStringAny purges struct data by serializing to JSON and then Unmarshal
// This is useful for templating because it can get hung up on types otherwise
func ConvertStructToMapStringAny(someStruct any) (map[string]any, error) {
	jsonBuffer, err := json.Marshal(someStruct)
	if err != nil {
		return map[string]any{}, err
	}
	mapData := map[string]any{}

	if err := json.Unmarshal(jsonBuffer, &mapData); err != nil {
		return map[string]any{}, err
	}

	return mapData, nil
}
