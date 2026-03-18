#!/bin/sh
# Fix ownership of the bind-mounted /data directory so the weather user
# (UID 1000) can write the SQLite database regardless of the host UID.
chown -R weather:weather /data
exec su-exec weather "$@"
