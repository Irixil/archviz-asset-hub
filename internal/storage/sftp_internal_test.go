package storage

import (
	"errors"
	"testing"

	gsftp "github.com/pkg/sftp"
)

// TestSFTPIsNotExist exercises the isNotExist helper directly, since there is
// no fake/mock SFTP server available in this module (no such test double is
// vendored — see go.mod) to drive Get() against a real not-found response
// from the wire. isNotExist is the exact predicate Get uses to decide whether
// to wrap the native error with ErrNotFound, so this pins its behavior for
// the status code that matters (SSH_FX_NO_SUCH_FILE) plus a couple of
// negative cases (a different status code, and a non-StatusError error).
// See ROADMAP.73 ST-2.3.
func TestSFTPIsNotExist(t *testing.T) {
	t.Run("no such file", func(t *testing.T) {
		err := &gsftp.StatusError{Code: uint32(gsftp.ErrSSHFxNoSuchFile)}
		if !isNotExist(err) {
			t.Errorf("isNotExist(%v) = false, want true", err)
		}
	})

	t.Run("other status code", func(t *testing.T) {
		err := &gsftp.StatusError{Code: uint32(gsftp.ErrSSHFxPermissionDenied)}
		if isNotExist(err) {
			t.Errorf("isNotExist(%v) = true, want false", err)
		}
	})

	t.Run("non status error", func(t *testing.T) {
		err := errors.New("boom")
		if isNotExist(err) {
			t.Errorf("isNotExist(%v) = true, want false", err)
		}
	})

	t.Run("nil error", func(t *testing.T) {
		if isNotExist(nil) {
			t.Error("isNotExist(nil) = true, want false")
		}
	})
}
