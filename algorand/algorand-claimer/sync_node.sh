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

# The developers recoommend just to look at sync time for full sync
# https://developer.algorand.org/docs/run-a-node/operations/catchup/
function is_node_synced {
  goal node status   |\
    grep "Sync Time"   |\
    awk '{ print $3 }' |\
    [ "$(</dev/stdin)" = "0.0s" ]
}

statusline "Doing a little setup check ..."
[[ -z "$ALGORAND_PASSPHRASE" ]] && echo "Environment variable ALGORAND_PASSPHRASE not defined or empty, exiting ..."

statusline "Ensuring connection to algorand node ...  (っ•́｡•́)♪♬"
goal node wait
statusline "Connection success!"

statusline "Catching up node to latest catchpoint ..."
goal node catchup $(curl https://algorand-catchpoints.s3.us-east-2.amazonaws.com/channel/mainnet/latest.catchpoint)

statusline "Waiting for node to be synced ..."
counter=0
max_retry=1
sleep_seconds=10
until [ "$(is_node_synced ; echo $?)" -eq "0" ]
do
   sleep $sleep_seconds
   [[ counter -eq $max_retry ]] && err_noexit "Failed to sync node, exiting ..." && exit 0
   statusline "Node not fully synced - $(($max_retry-$counter)) attempts left ..."
   ((counter++))
done
