package internal

import "strings"

func ImageSpec(repository, tagOrSha string) string {
	if strings.Contains(tagOrSha, ":") {
		return repository + "@" + tagOrSha
	}

	return repository + ":" + tagOrSha
}
