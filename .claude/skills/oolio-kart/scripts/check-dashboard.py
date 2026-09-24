#!/usr/bin/env python3
"""Runs every panel query of the Grafana dashboard against a Prometheus server
and reports which ones return no data. Run it after `make traffic`.

Usage: check-dashboard.py [dashboard.json] [prometheus-url]
Defaults: deploy/grafana/dashboards/oolio-kart.json, http://localhost:9090
Exits 1 if any query is empty or fails.
"""
import json
import subprocess
import sys
import urllib.parse
import urllib.request

root = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True).stdout.strip() or "."
path = sys.argv[1] if len(sys.argv) > 1 else f"{root}/deploy/grafana/dashboards/oolio-kart.json"
prom = (sys.argv[2] if len(sys.argv) > 2 else "http://localhost:9090").rstrip("/")

# Dashboard variables, replaced with values Prometheus accepts directly.
VARS = {"$job": ".*", "$__rate_interval": "1m", "$__range": "15m"}

bad = 0
for panel in json.load(open(path))["panels"]:
    for target in panel.get("targets", []):
        expr = target["expr"]
        for k, v in VARS.items():
            expr = expr.replace(k, v)
        url = f"{prom}/api/v1/query?" + urllib.parse.urlencode({"query": expr})
        try:
            result = json.load(urllib.request.urlopen(url, timeout=10))["data"]["result"]
            status = "OK   " if result else "EMPTY"
        except Exception as err:  # bad PromQL or Prometheus down
            result, status = [], f"ERROR {err}"
        if not result:
            bad += 1
        legend = target.get("legendFormat") or ""
        print(f"{status} {panel['title'][:30]:30} {legend[:18]}")

print(f"\n{bad} of the queries returned nothing" if bad else "\nall queries returned data")
sys.exit(1 if bad else 0)
