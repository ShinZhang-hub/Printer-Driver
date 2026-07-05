//go:build !windows

package drvpack

func tryElevatedExtract(exe, dir string) bool {
	return false
}
