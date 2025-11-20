#!/bin/sh -e

# Source debconf library.
. /usr/share/debconf/confmodule
db_version 2.0

db_title SkyPanel Installation

db_input medium SkyPanel/install_type || true
db_go

db_get SkyPanel/install_type
if [ "$RET" = "panel" ]; then
  db_input medium SkyPanel/create_user || true
  db_go

  db_get SkyPanel/create_user
  if [ "$RET" = "true" ]; then
      until false; do
        db_input medium SkyPanel/email || true
        db_input medium SkyPanel/username || true
        db_input medium SkyPanel/password || true
        db_input medium SkyPanel/password_confirm
        if [ $? -eq 30 ]; then
          break
        fi
        db_go

        db_get SkyPanel/password
        expectedPw=$RET
        db_get SkyPanel/password_confirm
        confirmedPw=$RET

        if [ "$expectedPw" = "$confirmedPw" ]; then
          db_reset SkyPanel/password_confirm
          break
        fi
        db_reset db_reset SkyPanel/password
        db_input medium SkyPanel/password_mismatch
        if [ $? -eq 30 ]; then
          break
        fi
        db_go
      done
  fi
fi

