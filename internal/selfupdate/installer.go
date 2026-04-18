package selfupdate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

// ReplaceExecutable swaps the new binary into place atomically.
// Works while the target is currently executing (POSIX-only).
func ReplaceExecutable(newPath, targetPath string) error {
	if runtime.GOOS == "windows" {
		return errors.New("self-update not supported on windows")
	}
	if err := os.Chmod(newPath, 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		// Best-effort: strip quarantine if present. Ignore errors.
		_ = exec.Command("/usr/bin/xattr", "-d", "com.apple.quarantine", newPath).Run()
	}
	if err := os.Rename(newPath, targetPath); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	// Different-filesystem fallback: stage a sibling, then rename.
	siblingPath := targetPath + ".new"
	if err := copyFile(newPath, siblingPath); err != nil {
		return err
	}
	if err := os.Chmod(siblingPath, 0o755); err != nil {
		os.Remove(siblingPath)
		return err
	}
	if err := os.Rename(siblingPath, targetPath); err != nil {
		os.Remove(siblingPath)
		return err
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// PreflightWritable fails fast if the user can't replace the target binary.
func PreflightWritable(targetPath string) error {
	f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("cannot write to %s: permission denied\ntry: sudo agiler upgrade", targetPath)
		}
		return err
	}
	return f.Close()
}

// ResolveExecutable returns the real path of the running binary,
// with symlinks resolved.
func ResolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe, nil //nolint:nilerr // fall back to the unresolved path
	}
	return resolved, nil
}
