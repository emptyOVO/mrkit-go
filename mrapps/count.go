package main

import (
	"strconv"
	"strings"

	"github.com/emptyOVO/mrkit-go/worker"
)

// Map expects each input line in TSV format:
//
//	id\tbiz_key\tmetric
//
// It emits: key=biz_key, value=1.
func Map(filename string, contents string, ctx worker.MrContext) {
	for _, line := range strings.Split(strings.TrimSpace(contents), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		ctx.EmitIntermediate(parts[1], "1")
	}
}

// Reduce counts rows per biz_key.
func Reduce(key string, values []string, ctx worker.MrContext) {
	ctx.Emit(key, strconv.Itoa(len(values)))
}
