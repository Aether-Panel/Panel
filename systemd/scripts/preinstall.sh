#!/usr/bin/env sh

useradd --system --home-dir /var/lib/SkyPanel --create-home --user-group SkyPanel

exitCode=$?
[ $exitCode -eq 0 ] || [ $exitCode -eq 9 ] || exit $exitCode
