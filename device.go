package main

import (
	"fmt"
	"sync"

	"golang.org/x/sys/unix"
)

type deviceInfo struct {
	major uint32
	minor uint32
}

var deviceCache sync.Map

func resolveDevice(path string) (uint32, uint32, error) {
	if v, ok := deviceCache.Load(path); ok {
		info := v.(deviceInfo)
		return info.major, info.minor, nil
	}

	var stat unix.Stat_t
	if err := unix.Stat(path, &stat); err != nil {
		return 0, 0, fmt.Errorf("stat %s: %w", path, err)
	}

	if stat.Mode&unix.S_IFMT != unix.S_IFBLK {
		return 0, 0, fmt.Errorf("%s is not a block device", path)
	}

	major := unix.Major(stat.Rdev)
	minor := unix.Minor(stat.Rdev)

	deviceCache.Store(path, deviceInfo{major: major, minor: minor})
	return major, minor, nil
}
