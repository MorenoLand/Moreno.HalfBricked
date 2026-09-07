# Web host files

Run `scripts/build.ps1 -Target wasm` from the project root. The generated `bin/web` directory contains `index.html`, `wasm_exec.js`, `aoz.wasm`, and the local generated `data` pack when one exists.

Serve `bin/web` through an HTTP server. Opening `index.html` directly from a file URL is not supported because browsers restrict WebAssembly and fetch requests there.
