# Config-Driven Flow

This project uses a config-driven pipeline:

- `source`
- `transform`
- `sink`

Main command:

```bash
go run ./cmd/batch -check -config /path/to/flow.json
go run ./cmd/batch -config /path/to/flow.json
```

## Config Sections

- `version`: currently `v1`
- `source`: mysql or redis source config
- `transform`: `builtin` or `mapreduce`
- `sink`: mysql or redis sink config

## Example (Production-Oriented Template)

```json
{
  "version": "v1",
  "source": {
    "type": "mysql",
    "db": {
      "host": "10.20.1.11",
      "port": 3306,
      "user": "etl_reader",
      "password": "REPLACE_ME",
      "database": "biz_source",
      "params": {
        "readTimeout": "60s",
        "writeTimeout": "60s",
        "timeout": "10s",
        "loc": "Local",
        "multiStatements": "false"
      }
    },
    "config": {
      "table": "order_events",
      "pkcolumn": "id",
      "keycolumn": "biz_key",
      "valcolumn": "metric",
      "where": "event_time >= '2026-02-01 00:00:00' AND event_time < '2026-02-02 00:00:00'",
      "shards": 64,
      "parallel": 16,
      "outputdir": "txt/mysql_source",
      "fileprefix": "chunk"
    }
  },
  "transform": {
    "type": "builtin",
    "builtin": "count",
    "reducers": 16,
    "workers": 32,
    "in_ram": false,
    "port": 18000
  },
  "sink": {
    "type": "mysql",
    "db": {
      "host": "10.20.2.15",
      "port": 3306,
      "user": "etl_writer",
      "password": "REPLACE_ME",
      "database": "biz_dw",
      "params": {
        "readTimeout": "60s",
        "writeTimeout": "60s",
        "timeout": "10s",
        "loc": "Local",
        "multiStatements": "false"
      }
    },
    "config": {
      "targettable": "order_metric_daily",
      "keycolumn": "biz_key",
      "valcolumn": "metric_sum",
      "inputglob": "mr-out-*.txt",
      "replace": true,
      "batchsize": 5000
    }
  }
}
```

Run:

```bash
go run ./cmd/batch -config /absolute/path/flow.prod.json
```

## Built-in Cross-DB Examples

- mysql -> mysql:
  - `example/batch-minimal/flows/smoke/flow.mysql.count.json`
- mysql -> redis:
  - `example/batch-minimal/flows/cross-db/flow.mysql_to_redis.count.json`
- redis -> mysql:
  - `example/batch-minimal/flows/cross-db/flow.redis_to_mysql.count.json`
- redis -> redis:
  - `example/batch-minimal/flows/cross-db/flow.redis_to_redis.count.json`

For `redis -> *` examples, preload `db0/event:*` source keys first:

```bash
go run ./cmd/batch -config example/batch-minimal/flows/seed/flow.seed.redis_source_event.json
```

## Rerun Suggestions

- Use a deterministic `where` window (time range / batch id).
- For idempotent reruns, set `sink.config.replace=true`.
- For accumulation mode, set `replace=false` and avoid duplicate source windows.
- Before rerun, clean local artifacts:

```bash
rm -f mr-out-*.txt txt/mysql_source/chunk-*.txt output/imd-*.txt
```

- Start with moderate parallelism:
  - `shards = 4x~8x reducers`
  - `workers = 2x reducers`
