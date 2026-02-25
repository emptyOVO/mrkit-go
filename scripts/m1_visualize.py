#!/usr/bin/env python3
import csv
import json
import os
import sys
from pathlib import Path


def usage() -> int:
    print("usage: scripts/m1_visualize.py <run_id|report_dir>", file=sys.stderr)
    return 2


def parse_run_dir(arg: str) -> Path:
    p = Path(arg)
    if p.is_dir():
        return p
    return Path("reports") / "m1" / arg


def load_json(path: Path):
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def load_ndjson(path: Path):
    out = []
    with path.open("r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            out.append(json.loads(line))
    return out


def write_scenario_csv(summary: dict, out_path: Path):
    rows = summary.get("per_scenario", [])
    with out_path.open("w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["scenario", "total", "passed", "failed", "avg_ms"])
        for r in rows:
            w.writerow([r.get("scenario"), r.get("total"), r.get("passed"), r.get("failed"), r.get("avg_ms")])


def write_details_csv(events: list, out_path: Path):
    with out_path.open("w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["ts", "round", "scenario", "status", "duration_ms", "message"])
        for e in events:
            w.writerow([e.get("ts"), e.get("round"), e.get("scenario"), e.get("status"), e.get("duration_ms"), e.get("message")])


def svg_bar(labels, values, title: str, y_suffix: str = "") -> str:
    width = 920
    height = 360
    margin_l = 60
    margin_r = 30
    margin_t = 40
    margin_b = 90
    chart_w = width - margin_l - margin_r
    chart_h = height - margin_t - margin_b
    n = max(1, len(values))
    bar_w = chart_w / n * 0.6
    gap = chart_w / n * 0.4
    vmax = max(values) if values else 1
    vmax = max(vmax, 1)

    parts = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}">',
        '<style>text{font-family:Arial,Helvetica,sans-serif;font-size:12px;fill:#1f2937}.title{font-size:16px;font-weight:700}.bar{fill:#3b82f6}.axis{stroke:#9ca3af;stroke-width:1}</style>',
        f'<text x="{margin_l}" y="24" class="title">{title}</text>',
        f'<line x1="{margin_l}" y1="{margin_t+chart_h}" x2="{margin_l+chart_w}" y2="{margin_t+chart_h}" class="axis"/>',
        f'<line x1="{margin_l}" y1="{margin_t}" x2="{margin_l}" y2="{margin_t+chart_h}" class="axis"/>',
    ]

    for i, (lab, val) in enumerate(zip(labels, values)):
        x = margin_l + i * (bar_w + gap) + gap / 2
        h = 0 if vmax == 0 else chart_h * (val / vmax)
        y = margin_t + chart_h - h
        parts.append(f'<rect class="bar" x="{x:.1f}" y="{y:.1f}" width="{bar_w:.1f}" height="{h:.1f}"/>')
        parts.append(f'<text x="{x + bar_w/2:.1f}" y="{margin_t+chart_h+18}" text-anchor="middle">{lab}</text>')
        parts.append(f'<text x="{x + bar_w/2:.1f}" y="{y-6:.1f}" text-anchor="middle">{val}{y_suffix}</text>')

    parts.append("</svg>")
    return "\n".join(parts)


def write_report_md(run_dir: Path, summary: dict, counts: dict):
    out = run_dir / "visualization.md"
    with out.open("w", encoding="utf-8") as f:
        f.write("# M1 Visualization\n\n")
        f.write(f"- run_id: `{summary.get('run_id')}`\n")
        f.write(f"- pass_rate_pct: `{summary.get('pass_rate_pct')}`\n")
        f.write(f"- total_cases: `{summary.get('total_cases')}`\n")
        f.write(f"- failed: `{summary.get('failed')}`\n\n")
        f.write("## Final Counts\n\n")
        f.write("| expected | m2m | r2m | seed | m2r | r2r |\n")
        f.write("|---:|---:|---:|---:|---:|---:|\n")
        f.write(
            f"| {counts.get('expected')} | {counts.get('m2m_rows')} | {counts.get('r2m_rows')} | {counts.get('seed_keys')} | {counts.get('m2r_keys')} | {counts.get('r2r_keys')} |\n\n"
        )
        f.write("## Charts\n\n")
        f.write(f"![Scenario Pass Rate](./scenario_pass_rate.svg)\n\n")
        f.write(f"![Scenario Avg Latency](./scenario_avg_ms.svg)\n")


def main() -> int:
    if len(sys.argv) != 2:
        return usage()

    run_dir = parse_run_dir(sys.argv[1])
    summary_path = run_dir / "summary.json"
    details_path = run_dir / "details.ndjson"
    counts_path = run_dir / "final_counts.json"

    for p in (summary_path, details_path, counts_path):
        if not p.exists():
            print(f"missing required report file: {p}", file=sys.stderr)
            return 2

    summary = load_json(summary_path)
    events = load_ndjson(details_path)
    counts = load_json(counts_path)

    viz_dir = run_dir / "viz"
    viz_dir.mkdir(parents=True, exist_ok=True)

    write_scenario_csv(summary, viz_dir / "scenario_summary.csv")
    write_details_csv(events, viz_dir / "details.csv")

    scenarios = summary.get("per_scenario", [])
    labels = [s.get("scenario", "") for s in scenarios]

    pass_rates = []
    for s in scenarios:
        total = max(1, int(s.get("total", 0)))
        passed = int(s.get("passed", 0))
        pass_rates.append(round(passed * 100.0 / total, 2))

    avg_ms = [int(s.get("avg_ms", 0)) for s in scenarios]

    (viz_dir / "scenario_pass_rate.svg").write_text(svg_bar(labels, pass_rates, "M1 Scenario Pass Rate", "%"), encoding="utf-8")
    (viz_dir / "scenario_avg_ms.svg").write_text(svg_bar(labels, avg_ms, "M1 Scenario Avg Duration (ms)"), encoding="utf-8")

    write_report_md(viz_dir, summary, counts)

    print(f"[viz] run_dir={run_dir}")
    print(f"[viz] generated: {viz_dir / 'scenario_summary.csv'}")
    print(f"[viz] generated: {viz_dir / 'details.csv'}")
    print(f"[viz] generated: {viz_dir / 'scenario_pass_rate.svg'}")
    print(f"[viz] generated: {viz_dir / 'scenario_avg_ms.svg'}")
    print(f"[viz] generated: {viz_dir / 'visualization.md'}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
