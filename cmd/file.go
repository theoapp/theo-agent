package cmd

import (
	"os"
	"path/filepath"
)

// writeFileAtomic replaces path in a single step, so readers and crashes never
// see a partial file. A uid or gid of -1 leaves that value unchanged. When
// validate is not nil it receives the complete temporary file, and an error
// leaves path untouched.
func writeFileAtomic(path string, data []byte, perm os.FileMode, uid int, gid int, validate func(string) error) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if err = f.Chown(uid, gid); err != nil {
		return err
	}
	if err = f.Chmod(perm); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if validate != nil {
		if err = validate(tmp); err != nil {
			return err
		}
	}
	return os.Rename(tmp, path)
}
