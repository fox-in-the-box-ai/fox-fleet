#!/bin/sh
set -e

# When running as root (e.g. docker run --user root), fix data dir
# ownership and drop to the nonroot user.
if [ "$(id -u)" = "0" ]; then
    chown 65532:65532 /var/lib/fox-control
    exec su-exec 65532:65532 "$@"
fi

exec "$@"
