#!/bin/bash
# Wait until system is up an running before returning
# see: https://blog.simos.info/how-to-know-when-a-lxd-container-has-finished-starting-up/
while [ "$(systemctl is-system-running 2>/dev/null)" != running ] \
&& [ "$(systemctl is-system-running 2>/dev/null)" != degraded ]
do
  :
done
# Linger starts the user manager for the specified user on boot, which then creates /run/user/$UID,
# sets $XDG_RUNTIME_DIR and more. Interfaces such as desktop rely on both of these to be present.
# This does not introduce any additional modification beyond what a login session would normally create.
loginctl enable-linger workshop
