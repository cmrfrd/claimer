#!/usr/bin/env bash

function catchup {
  LATEST_CATCHPOINT=$(curl -sq https://algorand-catchpoints.s3.us-east-2.amazonaws.com/channel/mainnet/latest.catchpoint)
  ACTIVE_CATCHPOINT=$(curl -sb -H "Accept: application/json" -H "X-Algo-API-Token: ${ALGORAND_CLAIMER_ALGOD_TOKEN}" http://localhost:${ALGORAND_CLAIMER_ALGOD_PORT}/v2/status | jq -r '.catchpoint')
  if [ "${LATEST_CATCHPOINT}" -eq "${ACTIVE_CATCHPOINT}" ]
  then
    echo "Node actively catching up with latest catchpoint ..."
  else
    echo "Aborting current catchup ..."
    goal node catchup -x
    echo "Catching node up to ${LATEST_CATCHPOINT} ..."
    goal node catchup ${LATEST_CATCHPOINT}
  fi
}

# catchup the algod node to a recent block
echo "Catching up algod node ..."
counter=0
max_retry=10
sleep_seconds=1
until [ "$(catchup ; echo $?)" -eq "0" ]
do
   sleep $sleep_seconds
   [[ counter -eq $max_retry ]] && err_noexit "Failed to catchup node, exiting ..." && exit 0
   echo "Failure to catchup node - $(($max_retry-$counter)) attempts left ..."
   ((counter++))
done
echo "Successfully set catchpoint ..."
sleep infinity
