//go:build !windows

package installer

func suppressDriverUI() func() {
	return func() {}
}
