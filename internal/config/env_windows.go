//go:build windows

package config

import (
	"syscall"
	"unsafe"
)

const machineEnvironmentKey = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`

func lookupPersistentEnv(name string) (string, bool) {
	locations := []struct {
		root syscall.Handle
		path string
	}{
		{root: syscall.HKEY_CURRENT_USER, path: "Environment"},
		{root: syscall.HKEY_LOCAL_MACHINE, path: machineEnvironmentKey},
	}

	for _, location := range locations {
		if value, ok := readRegistryString(location.root, location.path, name); ok {
			return value, true
		}
	}
	return "", false
}

func readRegistryString(root syscall.Handle, path, name string) (string, bool) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", false
	}

	var key syscall.Handle
	if err := syscall.RegOpenKeyEx(root, pathPtr, 0, syscall.KEY_READ, &key); err != nil {
		return "", false
	}
	defer syscall.RegCloseKey(key)

	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", false
	}

	var valueType uint32
	var size uint32
	if err := syscall.RegQueryValueEx(key, namePtr, nil, &valueType, nil, &size); err != nil {
		return "", false
	}
	if size == 0 || (valueType != syscall.REG_SZ && valueType != syscall.REG_EXPAND_SZ) {
		return "", false
	}

	buffer := make([]uint16, (size+1)/2)
	if err := syscall.RegQueryValueEx(
		key,
		namePtr,
		nil,
		&valueType,
		(*byte)(unsafe.Pointer(&buffer[0])),
		&size,
	); err != nil {
		return "", false
	}
	return syscall.UTF16ToString(buffer), true
}
