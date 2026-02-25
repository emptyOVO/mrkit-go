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

// Reduce sums all metric values of each biz_key.
func Reduce(key string, values []string, ctx worker.MrContext) {
	total := 0
	for _, s := range values {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		total += n
	}
	ctx.Emit(key, strconv.Itoa(total))
}
