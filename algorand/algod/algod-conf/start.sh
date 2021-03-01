#!/usr/bin/env bash
/opt/node/bin/goal node start -d /opt/node/network/Node
/opt/node/bin/kmd start -t 0 -d /opt/node/network/Node/kmd-v0.5
sleep infinity
