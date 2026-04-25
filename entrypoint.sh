#!/bin/sh
chmod 644 /etc/resolv.conf 2>/dev/null || true

# Go's pure DNS resolver (CGO_ENABLED=0) uses [::1]:53 on IPv6-enabled hosts instead of
# 127.0.0.11. Pre-resolve Docker service names via musl and write to /etc/hosts so Go
# reads the hosts file and bypasses DNS entirely for these names.
for svc in db cache; do
    ip=$(nslookup "$svc" 2>/dev/null | grep '^Address: ' | awk '{print $2}')
    if [ -n "$ip" ]; then
        echo "$ip $svc" >> /etc/hosts
    fi
done

mkdir -p /app/output
chown appuser:appuser /app/output 2>/dev/null || true
OUTPUT_DIR="${OUTPUT_DIR:-/app/output}"
mkdir -p "$OUTPUT_DIR"
chown appuser:appuser "$OUTPUT_DIR" 2>/dev/null || true

exec su-exec appuser "$@"
