package batch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func buildPluginIfNeeded(workDir, sourcePath, outputPath string) error {
	lockPath := outputPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN) }()

	srcInfo, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if outInfo, err := os.Stat(outputPath); err == nil && !outInfo.ModTime().Before(srcInfo.ModTime()) {
		return nil
	}

	tmpOutput := filepath.Join(filepath.Dir(outputPath), "."+filepath.Base(outputPath)+".tmp."+strconv.Itoa(os.Getpid()))
	defer os.Remove(tmpOutput)

	cmd := exec.Command(goBinary(), "build", "-buildmode=plugin", "-o", tmpOutput, sourcePath)
	cmd.Dir = workDir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build plugin failed: %w (%s)", err, string(out))
	}
	if err := os.Rename(tmpOutput, outputPath); err != nil {
		return err
	}
	return nil
}

func goBinary() string {
	if v := os.Getenv("GO"); v != "" {
		return v
	}
	return "go"
}
