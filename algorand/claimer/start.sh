#!/usr/bin/env bash
set -e

default="\033[0m"
red="\033[0;31m"
green="\033[0;32m"
blue="\033[0;34m"
teal="\033[0;36m"
Bgreen="\033[1;32m"

function printc () {
  printf "$1$2${default}\n"
}

function statusline () {
  printc "${Bgreen}" "\n$1"
}

function err_noexit () {
  printf "${red}$1${default}\n"
}

function err () {
  err_noexit "$1"
  exit 1
}

function shutdown_node {
  curl \
    -X POST \
    -H "X-Algo-API-Token: ${ALGORAND_CLAIMER_ALGOD_TOKEN}" \
    http://localhost:${SHUTDOWN_PORT}
}

# The developers recoommend just to look at sync time for full sync
# https://developer.algorand.org/docs/run-a-node/operations/catchup/
function is_node_synced {
  curl \
    -q \
    -H "Accept: application/json" \
    -H "X-Algo-API-Token: ${ALGORAND_CLAIMER_ALGOD_TOKEN}" \
    http://localhost:${ALGORAND_CLAIMER_ALGOD_PORT}/v2/status | \
    jq -e '."catchup-time" == 0' > /dev/null
}

echo "Waiting for algod ..."
timeout 22 sh -c 'until nc -z $0 $1; do sleep 1; done' localhost ${ALGORAND_CLAIMER_ALGOD_PORT}

statusline "Waiting for node to be synced ... (っ•́｡•́)♪♬"
counter=0
max_retry=240
sleep_seconds=15
until [ "$(is_node_synced ; echo $?)" -eq "0" ]
do
   sleep $sleep_seconds
   if [[ counter -eq $max_retry ]]
   then
      err_noexit "Failed to sync node, exiting ..."
      exit 0
   fi
   statusline "Node not fully synced - $(($max_retry-$counter)) attempts left ..."
   ((counter++))
done
statusline "Node synced!"

statusline "Executing claimer ..."
/algorand-claimer
statusline "Claimer completed ..."

statusline "Shutting down algod node ..."
shutdown_node
statusline "Node shut down, exiting ..."
