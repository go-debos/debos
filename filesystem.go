package debos

import (
	"fmt"
	"io"
	"io/ioutil"
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
		return err
	}
	defer in.Close()
	tmp, err := ioutil.TempFile(filepath.Dir(dst), "")
	if err != nil {
		return err
	}
	_, err = io.Copy(tmp, in)
	if err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err = tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err = os.Chmod(tmp.Name(), mode); err != nil {
		os.Remove(tmp.Name())
		return err
	}

	if err = os.Rename(tmp.Name(), dst); err != nil {
		os.Remove(tmp.Name())
		return err
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
				return fmt.Errorf("Failed to copy file %s: %w", p, err)
			}
		case os.ModeDir:
			os.Mkdir(target, info.Mode())
		case os.ModeSymlink:
			link, err := os.Readlink(p)
			if err != nil {
				return fmt.Errorf("Failed to read symlink %s: %w", suffix, err)
			}
			os.Symlink(link, target)
		default:
			return fmt.Errorf("File %s with mode %v not handled", p, info.Mode())
		}

		return nil
	}

	return filepath.Walk(sourcetree, walker)
}

func RealPath(path string) (string, error) {
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}

	return filepath.Abs(p)
}

func RestrictedPath(prefix, dest string) (string, error) {
	var err error
	destination := path.Join(prefix, dest)
	destination, err = filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(destination, prefix) {
		return "", fmt.Errorf("The resulting path points outside of prefix '%s': '%s'\n", prefix, destination)
	}
	return destination, nil
}
