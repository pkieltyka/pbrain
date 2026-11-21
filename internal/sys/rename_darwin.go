//go:build darwin

package sys

import "golang.org/x/sys/unix"

// renameNoReplace renames within one folder and fails with EEXIST if the target exists.
func renameNoReplace(dirfd int, from, to string) error {
	return unix.RenameatxNp(dirfd, from, dirfd, to, unix.RENAME_EXCL)
}
