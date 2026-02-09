package main

import (
	"os"
	"sort"
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

// Reduce computes the N-th largest metric per biz_key.
// N is controlled by MYSQL_TOPN_N (default 3).
// When N > number of values, it returns the smallest available value.
func Reduce(key string, values []string, ctx worker.MrContext) {
	n := 3
	if s := strings.TrimSpace(os.Getenv("MYSQL_TOPN_N")); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			n = v
		}
	}

	nums := make([]int, 0, len(values))
	for _, s := range values {
		v, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		nums = append(nums, v)
	}
	if len(nums) == 0 {
		ctx.Emit(key, "0")
		return
	}

	sort.Slice(nums, func(i, j int) bool { return nums[i] > nums[j] })
	idx := n - 1
	if idx >= len(nums) {
		idx = len(nums) - 1
	}
	ctx.Emit(key, strconv.Itoa(nums[idx]))
}
