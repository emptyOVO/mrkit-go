package main

import (
	"os"
	"strconv"
	"strings"

	"github.com/emptyOVO/mrkit-go/worker"
)

// Map expects each input line in TSV format:
//
//	id\tbiz_key\tmetric
//
// It emits: key=biz_key, value=metric.
func Map(filename string, contents string, ctx worker.MrContext) {
	for _, line := range strings.Split(strings.TrimSpace(contents), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		ctx.EmitIntermediate(parts[1], parts[2])
	}
}

// Reduce computes min/max/range per biz_key.
// Mode is controlled by MYSQL_MINMAX_MODE:
// - min
// - max
// - range (max-min)
// default: max
func Reduce(key string, values []string, ctx worker.MrContext) {
	if len(values) == 0 {
		ctx.Emit(key, "0")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("MYSQL_MINMAX_MODE")))
	if mode == "" {
		mode = "max"
	}

	inited := false
	minV := 0
	maxV := 0
	for _, s := range values {
		v, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		if !inited {
			minV = v
			maxV = v
			inited = true
			continue
		}
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	if !inited {
		ctx.Emit(key, "0")
		return
	}

	out := maxV
	switch mode {
	case "min":
		out = minV
	case "range":
		out = maxV - minV
	default:
		out = maxV
	}
	ctx.Emit(key, strconv.Itoa(out))
}
