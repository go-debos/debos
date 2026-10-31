package debos

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// CleanPathAt returns path cleaned, resolved relative to at when it is not
// absolute.
func CleanPathAt(path, at string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	return filepath.Join(at, path)
}

// CleanPath returns path cleaned, resolved relative to the working directory
// when it is not absolute.
func CleanPath(path string) string {
	cwd, _ := os.Getwd()

	return CleanPathAt(path, cwd)
}

// CopyFile copies the file src to dst with the given mode, replacing dst
// atomically.
func CopyFile(src, dst string, mode os.FileMode) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", src, err)
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close %s: %w", src, closeErr))
		}
	}()
	tmp, err := os.CreateTemp(filepath.Dir(dst), "")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for %s: %w", dst, err)
	}
	defer func() {
		if err == nil {
			return
		}
		if removeErr := os.Remove(tmp.Name()); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to remove %s: %w", tmp.Name(), removeErr))
		}
	}()
	_, err = io.Copy(tmp, in)
	if err != nil {
		err = fmt.Errorf("failed to copy %s to %s: %w", src, tmp.Name(), err)
		if closeErr := tmp.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close %s: %w", tmp.Name(), closeErr))
		}

		return err
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", tmp.Name(), err)
	}
	if err = os.Chmod(tmp.Name(), mode); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", tmp.Name(), err)
	}

	if err = os.Rename(tmp.Name(), dst); err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", tmp.Name(), dst, err)
	}

	return nil
}

// CopyTree recursively copies the directory sourcetree to desttree.
func CopyTree(sourcetree, desttree string) error {
	walker := func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk %s: %w", p, err)
		}

		suffix, _ := filepath.Rel(sourcetree, p)
		target := path.Join(desttree, suffix)
		switch info.Mode() & os.ModeType {
		case 0:
			err := CopyFile(p, target, info.Mode())
			if err != nil {
				return fmt.Errorf("failed to copy file %s: %w", p, err)
			}
		case os.ModeDir:
			if err := os.Mkdir(target, info.Mode()); err != nil && !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
		case os.ModeSymlink:
			link, err := os.Readlink(p)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", suffix, err)
			}
			//nolint:gosec // Copies a tree debos controls; the symlink target is copied verbatim.
			if err := os.Symlink(link, target); err != nil && !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("failed to create symlink %s: %w", target, err)
			}
		default:
			return fmt.Errorf("file %s with mode %v not handled", p, info.Mode())
		}

		return nil
	}

	if err := filepath.Walk(sourcetree, walker); err != nil {
		return fmt.Errorf("failed to copy tree %s to %s: %w", sourcetree, desttree, err)
	}

	return nil
}

// RealPath returns the absolute path with all symbolic links resolved.
func RealPath(path string) (string, error) {
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate symlinks of %s: %w", path, err)
	}

	p, err = filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path of %s: %w", p, err)
	}

	return p, nil
}

// RestrictedPath joins dest to prefix and returns the result, or an error if
// it would point outside of prefix.
func RestrictedPath(prefix, dest string) (string, error) {
	var err error
	destination := path.Join(prefix, dest)
	destination, err = filepath.Abs(destination)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path of %s: %w", destination, err)
	}
	if !strings.HasPrefix(destination, prefix) {
		return "", fmt.Errorf("resulting path points outside of prefix '%s': '%s'", prefix, destination)
	}

	return destination, nil
}
