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
	local := make(map[string]int, 256)
	for _, line := range strings.Split(strings.TrimSpace(contents), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		local[parts[1]]++
	}
	for k, v := range local {
		ctx.EmitIntermediate(k, strconv.Itoa(v))
	}
}

// Reduce counts rows per biz_key.
func Reduce(key string, values []string, ctx worker.MrContext) {
	sum := 0
	for _, v := range values {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			// Backward compatibility with old "1 per value" behavior.
			sum++
			continue
		}
		sum += n
	}
	ctx.Emit(key, strconv.Itoa(sum))
}
