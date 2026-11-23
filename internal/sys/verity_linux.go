package sys

import (
	"errors"
	"io"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

var _ Verity = native{}

func (native) MeasureVerity(file RegularFile) ([32]byte, error) {
	r, ok := file.(regularFile)
	if !ok {
		return [32]byte{}, ErrVerityUnavailable
	}
	return measureVerity(r.File)
}

func measureVerity(f *os.File) ([32]byte, error) {
	var st unix.Statfs_t
	if err := unix.Fstatfs(int(f.Fd()), &st); err != nil {
		return [32]byte{}, err
	}
	switch st.Type {
	case unix.BTRFS_SUPER_MAGIC, unix.EXT4_SUPER_MAGIC, unix.F2FS_SUPER_MAGIC:
	default:
		return [32]byte{}, ErrVerityUnavailable
	}

	// The ioctl encodes the four-byte header size. Give the kernel room
	// for SHA-512 as well so a sealed file with that profile is distinguishable.
	var arg struct {
		unix.FsverityDigest
		sum [64]byte
	}
	arg.Size = uint16(len(arg.sum))
	err := ioctlVerity(f, unix.FS_IOC_MEASURE_VERITY, unsafe.Pointer(&arg))
	switch {
	case errors.Is(err, unix.ENODATA):
		return [32]byte{}, ErrVerityUnsealed
	case verityUnavailable(err):
		return [32]byte{}, ErrVerityUnavailable
	case errors.Is(err, unix.EOVERFLOW):
		return [32]byte{}, ErrVerityProfile
	case err != nil:
		return [32]byte{}, err
	case arg.Algorithm != unix.FS_VERITY_HASH_ALG_SHA256 || arg.Size != 32:
		return [32]byte{}, ErrVerityProfile
	default:
		return [32]byte(arg.sum[:32]), nil
	}
}

func (n native) SealTempVerity(dir Dir, name string, w io.WriteCloser) ([32]byte, error) {
	open := true
	defer func() {
		if open {
			_ = w.Close()
		}
	}()
	writer, ok := w.(*os.File)
	if !ok {
		return [32]byte{}, ErrVerityUnavailable
	}
	file, err := n.OpenRegular(dir, name)
	if err != nil {
		return [32]byte{}, err
	}
	r := file.(regularFile)
	defer r.Close()
	var sb unix.Stat_t
	if err := unix.Fstat(int(writer.Fd()), &sb); err != nil {
		return [32]byte{}, err
	}
	reading, err := r.Stat()
	if err != nil {
		return [32]byte{}, err
	}
	if !infoOf(&sb).Mode.IsRegular() || infoOf(&sb).Identity != reading.Identity {
		return [32]byte{}, ErrChanged
	}
	if _, err := measureVerity(r.File); !errors.Is(err, ErrVerityUnsealed) && err != nil {
		return [32]byte{}, err
	}
	open = false // Flush owns and closes the writer even on failure.
	if err := n.Flush(w); err != nil {
		return [32]byte{}, err
	}
	arg := unix.FsverityEnableArg{Version: 1, Hash_algorithm: unix.FS_VERITY_HASH_ALG_SHA256, Block_size: 4096}
	err = ioctlVerity(r.File, unix.FS_IOC_ENABLE_VERITY, unsafe.Pointer(&arg))
	if err != nil && !errors.Is(err, unix.EEXIST) {
		if verityUnavailable(err) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EROFS) ||
			errors.Is(err, unix.ETXTBSY) || errors.Is(err, unix.EPERM) {
			return [32]byte{}, ErrVerityUnavailable
		}
		return [32]byte{}, err
	}
	if err := r.Sync(); err != nil {
		return [32]byte{}, err
	}
	return measureVerity(r.File)
}

func verityUnavailable(err error) bool {
	return errors.Is(err, unix.ENOTTY) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS)
}

func ioctlVerity(file *os.File, request uint, arg unsafe.Pointer) error {
	err := retry(func() error {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, file.Fd(), uintptr(request), uintptr(arg))
		if errno != 0 {
			return errno
		}
		return nil
	})
	runtime.KeepAlive(file)
	return err
}
