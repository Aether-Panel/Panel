#!/usr/bin/env sh

systemctl daemon-reload

mkdir -p /etc/SkyPanel /var/log/SkyPanel /var/lib/SkyPanel /var/www/SkyPanel
if [ ! -f "/var/lib/SkyPanel/database.db" ]; then
  touch /var/lib/SkyPanel/database.db
fi

SkyPanel --config=/etc/SkyPanel/config.json db upgrade
exitCode=$?
[ $exitCode -eq 0 ] || [ $exitCode -eq 9 ] || exit $exitCode

chown -R SkyPanel:SkyPanel /etc/SkyPanel /var/log/SkyPanel /var/lib/SkyPanel /var/www/SkyPanel

if command -v apparmor_parser >/dev/null 2>&1
then
    apparmor_parser -r /etc/apparmor.d/SkyPanel
fi

chmod o-rx /etc/SkyPanel
chmod o-rx /var/lib/SkyPanel
