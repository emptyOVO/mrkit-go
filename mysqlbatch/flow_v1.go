package mysqlbatch

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const FlowVersionV1 = "v1"

var builtinPluginBuildMu sync.Mutex

var builtinTransformSources = map[string]string{
	"count":  "mrapps/mysql_count.go",
	"minmax": "mrapps/mysql_minmax.go",
	"topn":   "mrapps/mysql_topn.go",
}

// ValidateFlowConfig validates v1 flow schema and required fields.
func ValidateFlowConfig(cfg FlowConfig) error {
	if strings.TrimSpace(cfg.Version) != FlowVersionV1 {
		return fmt.Errorf("unsupported version: %q (expected %q)", cfg.Version, FlowVersionV1)
	}
	if cfg.Source.Type != "mysql" {
		return fmt.Errorf("unsupported source.type: %s", cfg.Source.Type)
	}
	if cfg.Sink.Type != "mysql" {
		return fmt.Errorf("unsupported sink.type: %s", cfg.Sink.Type)
	}
	if cfg.Transform.Type != "mapreduce" && cfg.Transform.Type != "builtin" {
		return fmt.Errorf("unsupported transform.type: %s", cfg.Transform.Type)
	}

	if cfg.Source.DB.User == "" || cfg.Source.DB.Database == "" {
		return fmt.Errorf("source.db.user and source.db.database are required")
	}
	if cfg.Sink.DB.User == "" || cfg.Sink.DB.Database == "" {
		return fmt.Errorf("sink.db.user and sink.db.database are required")
	}
	if strings.TrimSpace(cfg.Source.Config.Table) == "" {
		return fmt.Errorf("source.config.table is required")
	}
	if strings.TrimSpace(cfg.Sink.Config.TargetTable) == "" {
		return fmt.Errorf("sink.config.targettable is required")
	}

	switch cfg.Transform.Type {
	case "mapreduce":
		if strings.TrimSpace(cfg.Transform.PluginPath) == "" {
			return fmt.Errorf("transform.plugin_path is required when transform.type=mapreduce")
		}
	case "builtin":
		if strings.TrimSpace(cfg.Transform.PluginPath) != "" {
			return fmt.Errorf("transform.plugin_path must be empty when transform.type=builtin")
		}
		norm := normalizeBuiltinName(cfg.Transform.Builtin)
		if _, ok := builtinTransformSources[norm]; !ok {
			return fmt.Errorf("unsupported transform.builtin: %q", cfg.Transform.Builtin)
		}
		if (norm == "minmax" || norm == "topn") && cfg.Transform.Reducers > 1 {
			return fmt.Errorf("transform.builtin=%s requires reducers=1 to keep non-additive aggregation correct", norm)
		}
	}

	if s, ok := cfg.Transform.Params["MYSQL_TOPN_N"]; ok && strings.TrimSpace(s) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid MYSQL_TOPN_N: %q", s)
		}
	}
	if s, ok := cfg.Transform.Params["MYSQL_MINMAX_MODE"]; ok && strings.TrimSpace(s) != "" {
		mode := strings.ToLower(strings.TrimSpace(s))
		if mode != "min" && mode != "max" && mode != "range" {
			return fmt.Errorf("invalid MYSQL_MINMAX_MODE: %q", s)
		}
	}
	return nil
}

func resolveTransformPlugin(cfg FlowTransformConfig) (string, error) {
	if cfg.Type == "mapreduce" {
		return cfg.PluginPath, nil
	}
	return ensureBuiltinPlugin(cfg.Builtin)
}

func normalizeBuiltinName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, "-", "")
	n = strings.ReplaceAll(n, "_", "")
	return n
}

func ensureBuiltinPlugin(name string) (string, error) {
	norm := normalizeBuiltinName(name)
	rel, ok := builtinTransformSources[norm]
	if !ok {
		return "", fmt.Errorf("unsupported transform.builtin: %q", name)
	}
	root, err := projectRoot()
	if err != nil {
		return "", err
	}
	src := filepath.Join(root, rel)
	if _, err := os.Stat(src); err != nil {
		return "", err
	}

	outDir := filepath.Join(root, ".cache", "mysqlbatch-builtins")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	out := filepath.Join(outDir, norm+".so")

	builtinPluginBuildMu.Lock()
	defer builtinPluginBuildMu.Unlock()
	return out, buildPluginIfNeeded(root, src, out)
}

func projectRoot() (string, error) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to resolve project path")
	}
	// current file is under <root>/mysqlbatch
	return filepath.Dir(filepath.Dir(current)), nil
}
