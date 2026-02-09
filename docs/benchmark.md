# Benchmark

Run benchmark in config-driven mode:

```bash
go run ./cmd/batch -mode benchmark -config example/batch-minimal/flows/benchmark/flow.benchmark.mysql_to_redis.count.json
go run ./cmd/batch -mode benchmark -config example/batch-minimal/flows/benchmark/flow.benchmark.redis_to_redis.count.json
```

Typical output:

```text
source=XXms transform=XXms sink=XXms total=XXms
```

Recommended for stability: use config-driven benchmark commands above.

Plugin benchmark in runner mode is environment-sensitive (local runtime + port behavior) and is not the recommended benchmark path in this project docs.
