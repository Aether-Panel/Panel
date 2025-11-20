#!/usr/bin/env sh

if [ -f "/var/lib/SkyPanel/database.db" ]; then
  cp -f /var/lib/SkyPanel/database.db /var/lib/SkyPanel/database-RESTORE.db
fi
