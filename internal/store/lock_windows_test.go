//go:build windows

package store

import "time"

func init() { lockWait = 500 * time.Millisecond }
