# Transforms

`mrkit-go` supports two transform modes:

- `builtin`: no plugin build required
- `mapreduce`: custom plugin (`.so`) mode

## Built-in Transforms

Supported values in `transform.builtin`:

- `count`
- `minmax`
- `topN`

Example:

```json
{
  "transform": {
    "type": "builtin",
    "builtin": "count",
    "reducers": 4,
    "workers": 8,
    "in_ram": false,
    "port": 10010
  }
}
```

Notes:

- `count` is additive and can run with multiple reducers.
- `minmax` and `topN` are non-additive; use `reducers=1` for correctness.

## Plugin Mode

Use plugin mode for custom logic:

```json
{
  "transform": {
    "type": "mapreduce",
    "plugin_path": "cmd/agg.so",
    "reducers": 8,
    "workers": 16,
    "in_ram": false,
    "port": 10000
  }
}
```

Build example plugin:

```bash
go build -buildmode=plugin -o cmd/agg.so ./mrapps/agg.go
```

If you build your own plugin, export standard `Map` and `Reduce` functions.
