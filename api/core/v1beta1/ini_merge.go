package v1beta1

import "strings"

// iniBlock is the run of lines belonging to one INI section. name and header are
// empty for the lines that precede the first section header.
type iniBlock struct {
	name   string
	header string
	body   []string
}

// mergeAdditionalConfigs folds additional into configMap. A filename the
// structured config already generated is merged section-aware: a section
// present in both keeps a single header, and a key present in both takes the
// value from additional. A filename that was not generated is taken verbatim.
func mergeAdditionalConfigs(configMap, additional map[string]string) {
	for filename, content := range additional {
		if existing, ok := configMap[filename]; ok {
			configMap[filename] = mergeIniContent(existing, content)
		} else {
			configMap[filename] = content
		}
	}
}

func mergeIniContent(generated, additional string) string {
	blocks := parseIniBlocks(generated)

	for _, add := range parseIniBlocks(additional) {
		i := indexOfIniBlock(blocks, add.name)
		if i < 0 {
			blocks = appendIniBlock(blocks, add)
			continue
		}
		for _, line := range add.body {
			blocks[i].body = mergeIniLine(blocks[i].body, line)
		}
	}

	return renderIniBlocks(blocks)
}

func parseIniBlocks(content string) []iniBlock {
	if content == "" {
		return nil
	}

	blocks := []iniBlock{{}}
	for _, line := range strings.Split(strings.TrimSuffix(content, "\n"), "\n") {
		if name, ok := iniSectionName(line); ok {
			blocks = append(blocks, iniBlock{name: name, header: line})
			continue
		}
		blocks[len(blocks)-1].body = append(blocks[len(blocks)-1].body, line)
	}
	return blocks
}

func indexOfIniBlock(blocks []iniBlock, name string) int {
	for i := range blocks {
		if blocks[i].name == name {
			return i
		}
	}
	return -1
}

// mergeIniLine replaces the value of an existing key, otherwise appends the
// line ahead of any trailing blank lines.
func mergeIniLine(body []string, line string) []string {
	key, ok := iniKey(line)
	if !ok {
		return appendIniBodyLine(body, line)
	}

	for i, existing := range body {
		if k, ok := iniKey(existing); ok && k == key {
			body[i] = line
			return body
		}
	}
	return appendIniBodyLine(body, line)
}

func appendIniBodyLine(body []string, line string) []string {
	at := len(body)
	for at > 0 && strings.TrimSpace(body[at-1]) == "" {
		at--
	}
	if at == len(body) {
		return append(body, line)
	}

	body = append(body, "")
	copy(body[at+1:], body[at:])
	body[at] = line
	return body
}

// appendIniBlock adds a new section, separating it with a blank line to match
// the spacing the structured render uses between sections.
func appendIniBlock(blocks []iniBlock, add iniBlock) []iniBlock {
	if len(blocks) > 0 {
		last := &blocks[len(blocks)-1]
		if n := len(last.body); n == 0 || strings.TrimSpace(last.body[n-1]) != "" {
			last.body = append(last.body, "")
		}
	}
	return append(blocks, add)
}

func renderIniBlocks(blocks []iniBlock) string {
	var lines []string
	for _, b := range blocks {
		if b.header != "" {
			lines = append(lines, b.header)
		}
		lines = append(lines, b.body...)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func iniSectionName(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 2 || !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return "", false
	}
	return strings.TrimSpace(trimmed[1 : len(trimmed)-1]), true
}

func iniKey(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return "", false
	}

	eq := strings.Index(trimmed, "=")
	if eq <= 0 {
		return "", false
	}
	return strings.TrimSpace(trimmed[:eq]), true
}
