package html

import (
	"fmt"
	"reflect"
	"strings"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/flightdeck/internal"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

// CRDConfigPage generates a configuration page from the Site CRD using reflection
func CRDConfigPage(site *positcov1beta1.Site, config *internal.ServerConfig) Node {
	return page("Config", config,
		Main(
			H2(Text("Site Configuration"), Class("text-3xl font-bold text-gray-800 dark:text-white mb-4")),
			Div(
				Class("text-sm text-gray-600 dark:text-gray-400 mb-4"),
				Text("This configuration is auto-generated from the Site Custom Resource Definition"),
			),
			Div(
				Class("container mx-auto py-8"),
				renderSiteSpec(&site.Spec),
			),
		),
	)
}

// renderSiteSpec renders the SiteSpec using reflection
func renderSiteSpec(spec *positcov1beta1.SiteSpec) Node {
	return Div(
		Class("space-y-6"),
		renderStruct(reflect.ValueOf(spec).Elem(), "SiteSpec", 0),
	)
}

// renderStruct recursively renders a struct and its fields
func renderStruct(v reflect.Value, name string, depth int) Node {
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil
	}

	var nodes []Node
	t := v.Type()

	// Add section header if not at root level
	if depth > 0 {
		headerClass := fmt.Sprintf("text-%s font-bold text-gray-800 dark:text-white mb-2", getHeaderSize(depth))
		nodes = append(nodes, H3(Text(formatFieldName(name)), Class(headerClass)))
	}

	// Group fields by category for better organization
	basicFields := []Node{}
	productFields := []Node{}
	advancedFields := []Node{}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		// Get field name from JSON tag if available
		fieldName := fieldType.Name
		if jsonTag := fieldType.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			}
			// Skip fields marked as omitempty if they're empty
			if len(parts) > 1 && strings.Contains(parts[1], "omitempty") && isZeroValue(field) {
				continue
			}
		}

		// Skip internal or empty structs
		if isInternalField(fieldName) || (field.Kind() == reflect.Struct && isEmptyStruct(field)) {
			continue
		}

		row := renderField(field, fieldName, fieldType.Type, depth+1)
		if row != nil {
			// Categorize fields
			if isProductField(fieldName) {
				productFields = append(productFields, row)
			} else if isAdvancedField(fieldName) {
				advancedFields = append(advancedFields, row)
			} else {
				basicFields = append(basicFields, row)
			}
		}
	}

	// Render sections
	if depth == 0 {
		// At root level, organize into sections
		if len(basicFields) > 0 {
			nodes = append(nodes,
				H3(Text("Basic Configuration"), Class("text-xl font-bold text-gray-800 dark:text-white mb-2")),
				Table(
					Class("table-auto w-full border-collapse border border-gray-300 dark:border-gray-700 mb-6"),
					TBody(basicFields...),
				),
			)
		}
		if len(productFields) > 0 {
			nodes = append(nodes,
				H3(Text("Product Configuration"), Class("text-xl font-bold text-gray-800 dark:text-white mb-2")),
				Table(
					Class("table-auto w-full border-collapse border border-gray-300 dark:border-gray-700 mb-6"),
					TBody(productFields...),
				),
			)
		}
		if len(advancedFields) > 0 {
			nodes = append(nodes,
				H3(Text("Advanced Configuration"), Class("text-xl font-bold text-gray-800 dark:text-white mb-2")),
				Table(
					Class("table-auto w-full border-collapse border border-gray-300 dark:border-gray-700 mb-6"),
					TBody(advancedFields...),
				),
			)
		}
	} else {
		// For nested structs, render all fields together
		allFields := append(append(basicFields, productFields...), advancedFields...)
		if len(allFields) > 0 {
			tableClass := "table-auto w-full border-collapse border border-gray-300 dark:border-gray-700"
			if depth > 0 {
				tableClass += " ml-4"
			}
			nodes = append(nodes,
				Table(
					Class(tableClass),
					TBody(allFields...),
				),
			)
		}
	}

	return Div(Class("space-y-4"), Group(nodes))
}

// renderField renders a single field as a table row or nested structure
func renderField(v reflect.Value, name string, t reflect.Type, depth int) Node {
	if !v.IsValid() || isZeroValue(v) {
		return nil
	}

	formattedName := formatFieldName(name)

	switch v.Kind() {
	case reflect.String:
		value := v.String()
		if value == "" {
			return nil
		}
		return createFieldRow(formattedName, value)

	case reflect.Int, reflect.Int32, reflect.Int64:
		if v.Int() == 0 {
			return nil
		}
		return createFieldRow(formattedName, fmt.Sprintf("%d", v.Int()))

	case reflect.Bool:
		// Only show bool fields if they're true
		if v.Bool() {
			return createFieldRow(formattedName, "true")
		}
		return nil

	case reflect.Map:
		if v.Len() == 0 {
			return nil
		}
		return createExpandableField(formattedName, renderMap(v))

	case reflect.Slice:
		if v.Len() == 0 {
			return nil
		}
		return createExpandableField(formattedName, renderSlice(v, depth))

	case reflect.Struct:
		// For nested structs, render them inline
		return createExpandableField(formattedName, renderStruct(v, name, depth))

	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return renderField(v.Elem(), name, t.Elem(), depth)

	default:
		// For other types, show the value as string if possible
		return createFieldRow(formattedName, fmt.Sprintf("%v", v.Interface()))
	}
}

// createFieldRow creates a simple table row for a field
func createFieldRow(name, value string) Node {
	return Tr(
		Td(Text(name), Class("text-left font-semibold text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-700 p-2")),
		Td(Text(value), Class("text-left text-gray-800 dark:text-white border border-gray-300 dark:border-gray-700 p-2 font-mono")),
	)
}

// createExpandableField creates a row with expandable content
func createExpandableField(name string, content Node) Node {
	if content == nil {
		return nil
	}

	return Tr(
		Td(Text(name), Class("text-left font-semibold text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-700 p-2 align-top")),
		Td(content, Class("text-left text-gray-800 dark:text-white border border-gray-300 dark:border-gray-700 p-2")),
	)
}

// renderMap renders a map as a definition list
func renderMap(v reflect.Value) Node {
	if v.Len() == 0 {
		return nil
	}

	var items []Node
	for _, key := range v.MapKeys() {
		val := v.MapIndex(key)
		items = append(items,
			Div(
				Class("flex space-x-2"),
				Span(Text(fmt.Sprintf("%v:", key)), Class("font-semibold")),
				Span(Text(fmt.Sprintf("%v", val)), Class("font-mono")),
			),
		)
	}

	return Div(Class("space-y-1"), Group(items))
}

// renderSlice renders a slice as a list
func renderSlice(v reflect.Value, depth int) Node {
	if v.Len() == 0 {
		return nil
	}

	var items []Node
	isListOfPrimitives := false

	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)

		// For slices of structs, render each struct
		if elem.Kind() == reflect.Struct {
			items = append(items,
				Div(
					Class("border-l-2 border-gray-300 dark:border-gray-600 pl-2 ml-2 mb-2"),
					renderStruct(elem, fmt.Sprintf("Item %d", i+1), depth+1),
				),
			)
		} else {
			// For primitive types, render as list items
			isListOfPrimitives = true
			items = append(items,
				Li(Text(fmt.Sprintf("%v", elem.Interface())), Class("font-mono")),
			)
		}
	}

	if isListOfPrimitives {
		return Ul(Class("list-disc list-inside space-y-1"), Group(items))
	}
	return Div(Class("space-y-2"), Group(items))
}

// isZeroValue checks if a reflect.Value is the zero value for its type
func isZeroValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Map, reflect.Slice:
		return v.Len() == 0
	case reflect.Struct:
		// Check if all fields are zero values
		for i := 0; i < v.NumField(); i++ {
			if !isZeroValue(v.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	}
	return false
}

// isEmptyStruct checks if a struct has no exported non-zero fields
func isEmptyStruct(v reflect.Value) bool {
	if v.Kind() != reflect.Struct {
		return false
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		if t.Field(i).IsExported() && !isZeroValue(v.Field(i)) {
			return false
		}
	}
	return true
}

// isInternalField checks if a field name indicates it's internal
func isInternalField(name string) bool {
	// Skip Kubernetes internal fields
	return strings.HasPrefix(name, "XXX_") ||
		   name == "TypeMeta" ||
		   name == "ObjectMeta" ||
		   strings.HasSuffix(name, "_")
}

// isProductField checks if a field is a product configuration
func isProductField(name string) bool {
	productNames := []string{
		"workbench", "connect", "packageManager", "chronicle", "flightdeck",
	}
	for _, product := range productNames {
		if name == product {
			return true
		}
	}
	return false
}

// isAdvancedField checks if a field is an advanced configuration
func isAdvancedField(name string) bool {
	advancedNames := []string{
		"secret", "workloadSecret", "mainDatabaseCredentialSecret",
		"volumeSource", "volumeSubdirJobOff", "dropDatabaseOnTearDown",
		"debug", "logFormat", "networkTrust", "imagePullSecrets",
		"ingressAnnotations", "extraSiteServiceAccounts",
	}
	for _, advanced := range advancedNames {
		if name == advanced {
			return true
		}
	}
	return false
}

// formatFieldName converts a field name to a more readable format
func formatFieldName(name string) string {
	// Handle acronyms
	name = strings.ReplaceAll(name, "AWS", "AWS")
	name = strings.ReplaceAll(name, "VPC", "VPC")
	name = strings.ReplaceAll(name, "CIDR", "CIDR")
	name = strings.ReplaceAll(name, "EFS", "EFS")
	name = strings.ReplaceAll(name, "FQDN", "FQDN")

	// Add spaces before capitals (camelCase to Title Case)
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := name[i-1]
			if prev >= 'a' && prev <= 'z' {
				result.WriteString(" ")
			}
		}
		result.WriteRune(r)
	}

	return result.String()
}

// getHeaderSize returns the appropriate header size based on depth
func getHeaderSize(depth int) string {
	switch depth {
	case 1:
		return "2xl"
	case 2:
		return "xl"
	case 3:
		return "lg"
	default:
		return "base"
	}
}