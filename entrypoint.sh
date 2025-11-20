#!/usr/bin/env sh

/SkyPanel/bin/SkyPanel db migrate
exitCode=$?
[ $exitCode -eq 0 ] || [ $exitCode -eq 9 ] || exit $exitCode

/SkyPanel/bin/SkyPanel run