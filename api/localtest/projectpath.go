package localtest

// thanks to the discussion at
// https://stackoverflow.com/a/58294680/6570011

import (
	"path/filepath"
	"runtime"
)

var (
	_, b, _, _ = runtime.Caller(0)

	// RootDir the root folder of this project
	RootDir = filepath.Join(filepath.Dir(b), "../..")
)
