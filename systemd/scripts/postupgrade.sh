#!/usr/bin/env sh

systemctl daemon-reload

if [ ! -f "/var/lib/SkyPanel/database.db" ]; then
  touch /var/lib/SkyPanel/database.db
fi

systemctl is-active --quiet SkyPanel
wasRunning=$?

systemctl stop SkyPanel

SkyPanel --config=/etc/SkyPanel/config.json db upgrade
exitCode=$?
[ $exitCode -eq 0 ] || [ $exitCode -eq 9 ] || exit $exitCode

chown -R SkyPanel:SkyPanel /etc/SkyPanel /var/log/SkyPanel /var/lib/SkyPanel /var/www/SkyPanel
exitCode=$?
[ $exitCode -eq 0 ] || [ $exitCode -eq 9 ] || exit $exitCode

if [ $wasRunning -eq 0 ]; then
  systemctl restart SkyPanel
fi

exitCode=$?
[ $exitCode -eq 0 ] || [ $exitCode -eq 9 ] || exit $exitCode

if command -v apparmor_parser >/dev/null 2>&1
then
    apparmor_parser -r /etc/apparmor.d/SkyPanel
fi

chmod o-rx /etc/SkyPanel
chmod o-rx /var/lib/SkyPanel
