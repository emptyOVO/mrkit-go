# Benchmark

Run benchmark in config-driven mode.

Prerequisites:
- host machine with Go installed
- MySQL/Redis prepared according to your selected flow
- for Hadoop comparison: local `hadoop` command available on host

One-line Hadoop vs mrkit compare:

```bash
chmod +x scripts/benchmark_hadoop_compare.sh && ./scripts/benchmark_hadoop_compare.sh
```

If `hadoop` is missing, this script tries auto-install by default.
Set `AUTO_INSTALL_HADOOP=0` to disable auto-install.

For performance numbers, prefer prebuilt binary (avoids `go run` compile overhead):

```bash
go build -o ./bin/batch ./cmd/batch
./bin/batch -mode benchmark -config example/batch-minimal/flows/benchmark/flow.benchmark.mysql_to_redis.count.json
./bin/batch -mode benchmark -config example/batch-minimal/flows/benchmark/flow.benchmark.redis_to_redis.count.json
```

`go run` is fine for quick functional checks when precise throughput is not required.

Typical output:

```text
source=XXms transform=XXms sink=XXms total=XXms
```

Recommended for stability: run at least 3 rounds and use median.

Plugin benchmark in runner mode is environment-sensitive (local runtime + port behavior) and is not the recommended benchmark path in this project docs.
