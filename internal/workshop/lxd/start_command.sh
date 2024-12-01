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
# There is currently an assumption made in snapd that the $XAUTHORITY cookie is
# visible to confined snaps provided it's not located in /tmp. If it's present
# in /tmp, snapd will migrate the cookie to a directory that the snap can use.
# We cannot mount directly into /tmp via lxc to trigger this behavior, so we
# make a copy on startup instead
if [ -f /var/lib/workshop/run/Xauthority/.Xauthority ]; then
  /bin/cp -rf /var/lib/workshop/run/Xauthority/.Xauthority /tmp/.Xauthority
  chown workshop:workshop /tmp/.Xauthority
fi

wait_for_xauth() {
# This is a proof-of-concept hack.
# Installing on workshop launch is not ideal, especially as we have to perform
# an apt update call.
# One option would be to implement as part of the daemon, otherwise find a
# neater (and more thought out) way to implement via bash.
sudo apt update
sudo apt install -y inotify-tools
while true; do
inotifywait /var/lib/workshop/run
# There is currently an assumption made in snapd that the $XAUTHORITY cookie is
# visible to confined snaps provided it's not located in /tmp. If it's present
# in /tmp, snapd will migrate the cookie to a directory that the snap can use.
# We cannot mount directly into /tmp via lxc to trigger this behavior, so we
# make a copy on startup instead
if [ -f /var/lib/workshop/run/Xauthority/.Xauthority ]; then
  /bin/cp -rf /var/lib/workshop/run/Xauthority/.Xauthority /tmp/.Xauthority
  chown workshop:workshop /tmp/.Xauthority
fi
done
}
export -f wait_for_xauth
nohup bash -c wait_for_xauth &
