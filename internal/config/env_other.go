//go:build !windows

package config

func lookupPersistentEnv(string) (string, bool) {
	return "", false
}
