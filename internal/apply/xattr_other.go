//go:build !darwin

package apply

func copyXattrs(string, string) error { return nil }
