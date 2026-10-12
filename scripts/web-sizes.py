"""Writes sizes.json (path -> bytes) into a web data directory, next to pack.json.

The browser build reads it to show megabyte counts on the loading screen. Run it after exporting the data
directory:  python scripts/web-sizes.py G:/Software/nginx/www/_age/data
"""
import json
import os
import sys

root = sys.argv[1] if len(sys.argv) > 1 else "bin/data-cache"
sizes = {}
for folder, _, names in os.walk(root):
    for name in names:
        if name == "sizes.json":
            continue
        path = os.path.join(folder, name)
        sizes[os.path.relpath(path, root).replace(os.sep, "/")] = os.path.getsize(path)
with open(os.path.join(root, "sizes.json"), "w") as handle:
    json.dump(sizes, handle, separators=(",", ":"))
print(len(sizes), "files,", sum(sizes.values()) // (1 << 20), "MB")
