package worker

import "strings"

func encodeIMDKVs(kvs []KV) string {
	if len(kvs) == 0 {
		return ""
	}
	var b strings.Builder
	// Rough pre-size to reduce reallocations for hot path.
	b.Grow(len(kvs) * 24)
	for i := range kvs {
		b.WriteString(kvs[i].Key)
		b.WriteByte('\t')
		b.WriteString(kvs[i].Value)
		b.WriteByte('\n')
	}
	return b.String()
}

func decodeIMDKVs(raw string) []KV {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	out := make([]KV, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, KV{
			Key:   parts[0],
			Value: parts[1],
		})
	}
	return out
}
