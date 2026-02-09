package mysqlbatch

import (
	"fmt"
	"os"
	"os/exec"
)

func buildPluginIfNeeded(workDir, sourcePath, outputPath string) error {
	srcInfo, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if outInfo, err := os.Stat(outputPath); err == nil && !outInfo.ModTime().Before(srcInfo.ModTime()) {
		return nil
	}

	cmd := exec.Command(goBinary(), "build", "-buildmode=plugin", "-o", outputPath, sourcePath)
	cmd.Dir = workDir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build plugin failed: %w (%s)", err, string(out))
	}
	return nil
}

func goBinary() string {
	if v := os.Getenv("GO"); v != "" {
		return v
	}
	return "go"
}
