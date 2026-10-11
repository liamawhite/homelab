# Start with SQLite behind a replaceable storage interface

Melodical will run as a single Go container on a single-user homelab and share practice history across devices. Use SQLite initially to avoid operating a separate database service, but isolate persistence behind a storage interface because the user explicitly wants storage to be pluggable. A future backend must preserve practice history and review state semantics; it need not be selected or implemented in the first version.

Use a pure-Go SQLite driver and sqlc-generated queries inside the SQLite adapter. The runtime must build with `CGO_ENABLED=0` and require no C runtime; generated query types stay inside the adapter so they do not constrain future storage implementations.
