#!/bin/sh
PUID=${PUID:-1000}
PGID=${PGID:-1000}

addgroup -g "$PGID" bangou 2>/dev/null
adduser -u "$PUID" -G bangou -D -H bangou 2>/dev/null

# Fix ownership on data directory (db files)
chown -R "$PUID:$PGID" /data 2>/dev/null

exec su-exec bangou bangou "$@"
