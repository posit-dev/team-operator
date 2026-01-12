package templates

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Key         string       `json:"key,omitempty"`
	Num         int64        `json:"num,omitempty"`
	EmptyString string       `json:"emptyString,omitempty"`
	Nested      *otherStruct `json:"nested,omitempty"`
}

type otherStruct struct {
	String string `json:"string,omitempty"`
}

func TestConvertStructToMapStringAny(t *testing.T) {
	myStruct := testStruct{
		Key:         "hello",
		Num:         1234,
		EmptyString: "",
	}

	res, err := ConvertStructToMapStringAny(myStruct)
	require.Nil(t, err)
	require.Equal(t, "hello", res["key"])
	require.Equal(t, float64(1234), res["num"])
	require.Nil(t, res["emptyString"])
	require.Nil(t, res["nested"])
}

func TestGenerateGcfg(t *testing.T) {
	str, err := GenerateGcfg(map[string]any{
		"one": map[string]string{"hello": "there"},
		"two": map[string][]string{
			"hi": []string{"my", "friend"},
		},
		"Chicken \"Little\"": map[string]string{
			"fiddle": "diddle",
		},
	})

	require.Nil(t, err)
	require.Contains(t, str, "[one]")
	require.Contains(t, str, "hello = there")
	require.Contains(t, str, "[two]")
	require.Contains(t, str, "hi = my")
	require.Contains(t, str, "hi = friend")
	require.Contains(t, str, "[Chicken \"Little\"]")
	require.Contains(t, str, "fiddle = diddle")
}

func TestGenerateIni(t *testing.T) {
	myData := map[string]any{
		"parent": map[string]string{
			"key": "value",
		},
	}

	out, err := GenerateIni(myData)
	assert.Nil(t, err)
	assert.Contains(t, out, "[parent]")
	assert.Contains(t, out, "key=value")

	otherData := "string-asis"
	out, err = GenerateIni(otherData)
	assert.Nil(t, err)
	// it adds a newline... that's ok I guess...
	assert.Equal(t, "\nstring-asis", out)

	listData := []map[string]string{
		{"a": "b"},
		{"c": "d"},
	}
	out, err = GenerateIni(listData)
	assert.Nil(t, err)
	assert.Contains(t, out, "a=b")
	assert.Contains(t, out, "c=d")
}

func TestGenerateIniProfiles(t *testing.T) {
	profileData := map[string]any{
		"section": map[string]interface{}{
			"key": "value",
		},
		"*": map[string]interface{}{
			"test": "other",
			"one":  []string{"two", "three", "four"},
		},
	}

	out, err := GenerateIniProfiles(profileData)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	assert.Nil(t, err)
	assert.Contains(t, out, "[section]")
	assert.Contains(t, out, "key=value")
	assert.Contains(t, out, "test=other")
	assert.Contains(t, out, "one=two,three,four")
}

func TestRenderTemplateDataOutput(t *testing.T) {
	myData := map[string]any{
		"trailingDash": false, // this actually does nothing... so we should simplify our templates!
		"name":         "testing",
		"value": map[string]any{
			"one":  map[string]string{"two": "three"},
			"blue": map[string]bool{"tree": false},
		},
	}

	out, err := RenderTemplateDataOutput(myData)
	assert.Nil(t, err)
	assert.Contains(t, out, "{{- define \"testing\" }}")
	assert.Contains(t, out, "\"one\":{\"two\":\"three\"}")
	assert.Contains(t, out, "\"blue\":{\"tree\":false}")
}
