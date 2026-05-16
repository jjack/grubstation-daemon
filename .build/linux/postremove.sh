#!/bin/sh
set -e

# deb: $1 = "purge"
# rpm: $1 = 0
# apk: $1 = "remove" or "purge" (depends on how nfpms maps it)

if [ "$1" = "purge" ] || [ "$1" = "0" ]; then
    echo "Purging GrubStation configuration..."
    rm -rf /etc/grubstation
fi
