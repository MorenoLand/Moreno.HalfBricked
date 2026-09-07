# Security policy

Report security issues privately to the project maintainer before public disclosure. Do not include personal data, credentials, private content, extracted binaries, or generated caches in an issue.

The application accepts an explicit `--assets` path. It must not execute files from that directory, load native libraries, contact undocumented services, or bypass licensing and authentication systems.
