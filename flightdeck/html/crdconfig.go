package html

import (
	"fmt"
	"reflect"
	"slices"
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
			Class("max-w-7xl mx-auto py-8 px-4 sm:px-6 lg:px-8"),
			H2(Text("Site Configuration"), Class("text-3xl font-bold text-gray-800 dark:text-white mb-4")),
			Div(
				Class("text-sm text-gray-600 dark:text-gray-400 mb-6"),
				Text("This configuration is auto-generated from the Site Custom Resource Definition"),
			),
			renderSiteSpec(&site.Spec),
		),
	)
}

// renderSiteSpec renders the SiteSpec using reflection
func renderSiteSpec(spec *positcov1beta1.SiteSpec) Node {
	return renderStruct(reflect.ValueOf(spec).Elem(), "SiteSpec", 0)
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

		// For product fields at root level, render them differently
		if depth == 0 && isProductField(fieldName) {
			// Skip empty product structs
			if field.Kind() == reflect.Struct && isEmptyStruct(field) {
				continue
			}
			// Render product as a separate card section
			productSection := renderProductSection(field, fieldName)
			if productSection != nil {
				productFields = append(productFields, productSection)
			}
		} else {
			// Regular field rendering
			row := renderField(field, fieldName, fieldType.Type, depth+1)
			if row != nil {
				// Categorize fields
				if isAdvancedField(fieldName) {
					advancedFields = append(advancedFields, row)
				} else {
					basicFields = append(basicFields, row)
				}
			}
		}
	}

	// Render sections
	if depth == 0 {
		// At root level, organize into sections
		if len(basicFields) > 0 {
			nodes = append(nodes,
				Div(
					Class("mb-8"),
					H3(Text("Basic Configuration"), Class("text-2xl font-bold text-gray-800 dark:text-white mb-4")),
					Div(
						Class("bg-white dark:bg-neutral-800 rounded-md border border-posit-border dark:border-neutral-700 overflow-hidden"),
						Table(
							Class("table-auto w-full"),
							TBody(basicFields...),
						),
					),
				),
			)
		}
		if len(productFields) > 0 {
			nodes = append(nodes,
				Div(
					Class("mb-8"),
					H3(Text("Product Configuration"), Class("text-2xl font-bold text-gray-800 dark:text-white mb-4")),
					Div(Class("space-y-6"), Group(productFields)),
				),
			)
		}
		if len(advancedFields) > 0 {
			nodes = append(nodes,
				Div(
					Class("mb-8"),
					H3(Text("Advanced Configuration"), Class("text-2xl font-bold text-gray-800 dark:text-white mb-4")),
					Div(
						Class("bg-white dark:bg-neutral-800 rounded-md border border-posit-border dark:border-neutral-700 overflow-hidden"),
						Table(
							Class("table-auto w-full"),
							TBody(advancedFields...),
						),
					),
				),
			)
		}
	} else {
		// For nested structs, render all fields together
		allFields := slices.Concat(basicFields, productFields, advancedFields)
		if len(allFields) > 0 {
			nodes = append(nodes,
				Table(
					Class("table-auto w-full"),
					TBody(allFields...),
				),
			)
		}
	}

	return Div(Class("space-y-6"), Group(nodes))
}

// renderNestedStruct renders a nested struct as a clean subsection
func renderNestedStruct(v reflect.Value, name string, depth int) Node {
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil
	}

	var fieldItems []Node
	t := v.Type()

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

		// Skip internal or empty fields
		if isInternalField(fieldName) || isZeroValue(field) {
			continue
		}

		formattedName := formatFieldName(fieldName)

		// Dereference pointer fields
		actualField := field
		if actualField.Kind() == reflect.Ptr {
			if actualField.IsNil() {
				continue
			}
			actualField = actualField.Elem()
		}

		// Render the field value
		var valueNode Node
		switch actualField.Kind() {
		case reflect.String:
			valueNode = Span(Text(actualField.String()), Class("font-mono text-sm"))
		case reflect.Int, reflect.Int32, reflect.Int64:
			valueNode = Span(Text(fmt.Sprintf("%d", actualField.Int())), Class("font-mono text-sm"))
		case reflect.Bool:
			if actualField.Bool() {
				valueNode = Span(Text("true"), Class("font-mono text-sm"))
			}
		case reflect.Map:
			if actualField.Len() > 0 {
				valueNode = renderMap(actualField)
			}
		case reflect.Slice:
			if actualField.Len() > 0 {
				valueNode = renderSlice(actualField, depth+1)
			}
		case reflect.Struct:
			// Check for Raw JSON struct first
			display := formatValue(actualField)
			if display != "" && display != fmt.Sprintf("%v", actualField.Interface()) {
				valueNode = Span(Text(display), Class("font-mono text-sm"))
			} else {
				valueNode = renderNestedStruct(actualField, fieldName, depth+1)
			}
		default:
			valueNode = Span(Text(formatValue(actualField)), Class("font-mono text-sm"))
		}

		if valueNode != nil {
			fieldItems = append(fieldItems,
				Div(
					Class("flex flex-col sm:flex-row gap-2 py-2"),
					Div(Text(formattedName+":"), Class("font-semibold text-gray-700 dark:text-gray-300 min-w-[150px]")),
					Div(valueNode, Class("text-gray-800 dark:text-white")),
				),
			)
		}
	}

	if len(fieldItems) == 0 {
		return nil
	}

	return Div(
		Class("bg-gray-50 dark:bg-neutral-900 rounded p-3 space-y-1"),
		Group(fieldItems),
	)
}

// renderProductSection renders a product configuration as a card-style section
func renderProductSection(v reflect.Value, name string) Node {
	if !v.IsValid() || (v.Kind() == reflect.Struct && isEmptyStruct(v)) {
		return nil
	}

	formattedName := formatFieldName(name)

	// Collect all non-empty fields from the product struct
	var fieldRows []Node
	t := v.Type()

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

		// Skip internal fields
		if isInternalField(fieldName) {
			continue
		}

		// Render the field
		row := renderField(field, fieldName, fieldType.Type, 1)
		if row != nil {
			fieldRows = append(fieldRows, row)
		}
	}

	if len(fieldRows) == 0 {
		return nil
	}

	// Return a card-style section for this product
	return Div(
		Class("bg-white dark:bg-neutral-800 rounded-md border border-posit-border dark:border-neutral-700 p-6"),
		H4(Text(formattedName), Class("text-xl font-bold text-gray-800 dark:text-white mb-4")),
		Div(
			Class("overflow-x-auto"),
			Table(
				Class("table-auto w-full"),
				TBody(fieldRows...),
			),
		),
	)
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
		// For nested structs, render them as a nested section
		nestedContent := renderNestedStruct(v, name, depth)
		if nestedContent == nil {
			return nil
		}
		return createExpandableField(formattedName, nestedContent)

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
		Td(Text(name), Class("text-left font-semibold text-gray-700 dark:text-gray-300 border-b border-posit-border dark:border-neutral-700 p-3")),
		Td(Text(value), Class("text-left text-gray-800 dark:text-white border-b border-posit-border dark:border-neutral-700 p-3 font-mono text-sm")),
	)
}

// createExpandableField creates a row with expandable content
func createExpandableField(name string, content Node) Node {
	if content == nil {
		return nil
	}

	return Tr(
		Td(Text(name), Class("text-left font-semibold text-gray-700 dark:text-gray-300 border-b border-posit-border dark:border-neutral-700 p-3 align-top")),
		Td(content, Class("text-left text-gray-800 dark:text-white border-b border-posit-border dark:border-neutral-700 p-3")),
	)
}

// formatValue extracts a display string from a reflect.Value,
// handling special types like apiextensionsv1.JSON that contain raw JSON bytes,
// and dereferencing pointer types to their underlying values.
func formatValue(v reflect.Value) string {
	v = deref(v)
	if !v.IsValid() {
		return ""
	}

	// Check for structs with a Raw []byte field (e.g., apiextensionsv1.JSON)
	if v.Kind() == reflect.Struct {
		rawField := v.FieldByName("Raw")
		if rawField.IsValid() && rawField.Kind() == reflect.Slice && rawField.Type().Elem().Kind() == reflect.Uint8 {
			raw := rawField.Bytes()
			if len(raw) > 0 {
				s := strings.TrimSpace(string(raw))
				// Strip surrounding quotes from JSON strings
				if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
					s = s[1 : len(s)-1]
				}
				return s
			}
			return ""
		}
	}

	return fmt.Sprintf("%v", v.Interface())
}

// deref dereferences pointers and interfaces to their underlying value.
func deref(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

// renderMap renders a map, recursively expanding struct values
func renderMap(v reflect.Value) Node {
	if v.Len() == 0 {
		return nil
	}

	var items []Node
	for _, key := range v.MapKeys() {
		val := deref(v.MapIndex(key))
		if !val.IsValid() {
			continue
		}

		keyStr := fmt.Sprintf("%v", key.Interface())

		// If the value is a struct, render it recursively as a nested section
		if val.Kind() == reflect.Struct {
			nested := renderNestedStruct(val, keyStr, 2)
			if nested != nil {
				items = append(items,
					Div(
						Class("py-2"),
						Div(Text(keyStr+":"), Class("font-semibold text-gray-700 dark:text-gray-300 mb-1")),
						nested,
					),
				)
				continue
			}
		}

		// For scalar values, use formatValue
		display := formatValue(val)
		if display == "" {
			continue
		}
		items = append(items,
			Div(
				Class("flex gap-2 py-1"),
				Span(Text(keyStr+":"), Class("font-semibold text-gray-700 dark:text-gray-300")),
				Span(Text(display), Class("font-mono text-sm text-gray-800 dark:text-white")),
			),
		)
	}

	if len(items) == 0 {
		return nil
	}
	return Div(Class("bg-gray-50 dark:bg-neutral-900 rounded p-2 space-y-1"), Group(items))
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
					Class("bg-gray-50 dark:bg-neutral-900 rounded p-3 mb-2"),
					H5(Text(fmt.Sprintf("Item %d", i+1)), Class("font-semibold text-gray-700 dark:text-gray-300 mb-2")),
					renderNestedStruct(elem, fmt.Sprintf("Item %d", i+1), depth+1),
				),
			)
		} else {
			// For primitive types, render as list items
			isListOfPrimitives = true
			items = append(items,
				Li(Text(fmt.Sprintf("%v", elem.Interface())), Class("font-mono text-sm text-gray-800 dark:text-white")),
			)
		}
	}

	if isListOfPrimitives {
		return Div(
			Class("bg-gray-50 dark:bg-neutral-900 rounded p-2"),
			Ul(Class("list-disc list-inside space-y-1"), Group(items)),
		)
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
	// NOTE: This list is hardcoded. New product fields will default to the basic category
	// unless explicitly added here. This is intentional to ensure new fields are at least
	// visible rather than being lost.
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
	// NOTE: This list is hardcoded. New advanced fields will default to the basic category
	// unless explicitly added here. This ensures unknown fields are visible by default
	// rather than hidden in an advanced section.
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
