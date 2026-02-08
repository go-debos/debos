package debos

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func CleanPathAt(path, at string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	return filepath.Join(at, path)
}

func CleanPath(path string) string {
	cwd, _ := os.Getwd()
	return CleanPathAt(path, cwd)
}

func CopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source %s: %w", src, err)
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), "")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", filepath.Dir(dst), err)
	}
	if _, err = io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("copy to temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("close temp file: %w", err)
	}
	if err = os.Chmod(tmp.Name(), mode); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if err = os.Rename(tmp.Name(), dst); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("rename temp to dst %s: %w", dst, err)
	}

	return nil
}

func CopyTree(sourcetree, desttree string) error {
	walker := func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
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
			if err := os.Mkdir(target, info.Mode()); err != nil && !os.IsExist(err) {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
		case os.ModeSymlink:
			link, err := os.Readlink(p)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", suffix, err)
			}
			if err := os.Symlink(link, target); err != nil && !os.IsExist(err) {
				return fmt.Errorf("failed to create symlink %s: %w", target, err)
			}
		default:
			return fmt.Errorf("file %s with mode %v not handled", p, info.Mode())
		}

		return nil
	}

	return filepath.Walk(sourcetree, walker)
}

func RealPath(path string) (string, error) {
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("eval symlinks %s: %w", path, err)
	}

	if abs, err := filepath.Abs(p); err != nil {
		return "", fmt.Errorf("abs path %s: %w", p, err)
	} else {
		return abs, nil
	}
}

func RestrictedPath(prefix, dest string) (string, error) {
	var err error
	destination := path.Join(prefix, dest)
	destination, err = filepath.Abs(destination)
	if err != nil {
		return "", fmt.Errorf("abs path %s: %w", destination, err)
	}
	if !strings.HasPrefix(destination, prefix) {
		return "", fmt.Errorf("resulting path points outside of prefix '%s': '%s'", prefix, destination)
	}
	return destination, nil
}
