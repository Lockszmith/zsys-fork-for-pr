package daemon

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/ubuntu/zsys"
	"github.com/ubuntu/zsys/internal/config"
	"github.com/ubuntu/zsys/internal/log"
	"github.com/ubuntu/zsys/internal/zfs"
)

const (
	updateGrubCmd = "update-grub"
)

// PrepareBoot consolidates canmount states for early boot.
// Return if any dataset / machine changed has been done during boot and an error if any encountered.
func (s *Server) PrepareBoot(req *zsys.Empty, stream zsys.Zsys_PrepareBootServer) error {
	defer s.resetTimeout()

	z := zfs.New(stream.Context(), zfs.WithTransactions())

	var err error
	defer func() {
		if err != nil {
			z.Cancel()
			err = fmt.Errorf("couldn't ensure boot: "+config.ErrorFormat, err)
		} else {
			z.Done()
		}
	}()

	ms, err := getMachines(stream.Context(), z)
	if err != nil {
		return err
	}

	changed, err := ms.EnsureBoot(stream.Context(), z)
	if err != nil {
		return err
	}
	stream.Send(&zsys.PrepareBootResponse{
		Reply: &zsys.PrepareBootResponse_Changed{Changed: changed},
	})

	return nil
}

// CommitBoot commits current state to be the active one by promoting its datasets if needed, set last used,
// associate user datasets to it and rebuilding grub menu.
// After this operation, every New() call will get the current and correct system state.
// Return if any dataset / machine changed has been done during boot commit and an error if any encountered.
func (s *Server) CommitBoot(req *zsys.Empty, stream zsys.Zsys_CommitBootServer) error {
	defer s.resetTimeout()

	z := zfs.New(stream.Context(), zfs.WithTransactions())

	var err error
	defer func() {
		if err != nil {
			z.Cancel()
			err = fmt.Errorf("couldn't commit: "+config.ErrorFormat, err)
		} else {
			z.Done()
		}
	}()

	ms, err := getMachines(stream.Context(), z)
	if err != nil {
		return err
	}

	changed, err := ms.Commit(stream.Context(), z)
	if err != nil {
		return err
	}
	stream.Send(&zsys.CommitBootResponse{
		Reply: &zsys.CommitBootResponse_Changed{Changed: changed},
	})

	if !changed {
		return nil
	}

	cmd := exec.Command(updateGrubCmd)
	logger := &logWriter{ctx: stream.Context()}
	cmd.Stdout = logger
	cmd.Stderr = logger
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%q returned an error:"+config.ErrorFormat, updateGrubCmd, err)
	}

	return nil
}

type logWriter struct {
	ctx context.Context
}

func (lw logWriter) Write(p []byte) (n int, err error) {
	log.Debug(lw.ctx, string(p))
	return len(p), nil
}