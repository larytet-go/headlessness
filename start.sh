#!/bin/bash
set -e
set -x

export QT_X11_NO_MITSHM=1
export _X11_NO_MITSHM=1
export _MITSHM=0

# In case the container is restarted 
[ -f /tmp/.X99-lock ] && rm /tmp/.X99-lock


_kill_procs() {
  kill -TERM $pyppeteer
  kill -TERM $xvfb
}

# Relay quit commands to processes
trap _kill_procs SIGTERM SIGINT

export DISPLAY=:0
Xvfb $DISPLAY -screen 0 1024x768x16 -nolisten tcp -nolisten unix &
xvfb=$!

# https://linux.die.net/man/1/x11vnc
x11vnc -nopw -display $DISPLAY -N -forever > /dev/null &
x11vnc=$!

# Start chrome
./headlessness
pyppeteer=$!

wait $pyppeteer
# wait $xvfb
# wait $x11vnc