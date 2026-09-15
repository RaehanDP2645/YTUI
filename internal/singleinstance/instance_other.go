//go:build !windows

package singleinstance

// Acquire always returns false on non-Windows platforms so the
// application runs normally; single-instance guard is Windows-only.
func Acquire() bool { return false }

// Release is a no-op on non-Windows platforms.
func Release() {}

// ShowDialog is a no-op on non-Windows platforms.
func ShowDialog() {}

// FindAndRestore is a no-op on non-Windows platforms.
func FindAndRestore() {}
