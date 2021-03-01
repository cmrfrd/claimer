#!/usr/bin/env bash

function catchup {
  LATEST_CATCHPOINT=$(curl -sq https://algorand-catchpoints.s3.us-east-2.amazonaws.com/channel/mainnet/latest.catchpoint)
  LATEST_CATCHPOINT_ROUND=$(echo $LATEST_CATCHPOINT | cut -d"#" -f1)
  LAST_ROUND=$(curl -s -H "Accept: application/json" -H "X-Algo-API-Token: ${ALGORAND_CLAIMER_ALGOD_TOKEN}" http://localhost:${ALGORAND_CLAIMER_ALGOD_PORT}/v2/status | jq -r '."last-round"')
  BLOCK_DRIFT=`expr $LATEST_CATCHPOINT_ROUND - $LAST_ROUND`

  echo "Latest catchpoint round: ${LATEST_CATCHPOINT_ROUND}"
  echo "Current round: ${LAST_ROUND}"
  echo "Block drift: ${BLOCK_DRIFT}"

  if (( $BLOCK_DRIFT > $CATCHPOINT_BLOCK_DRIFT_THRESHOLD ))
  then
    echo "Aborting current catchup ..."
    goal node catchup -x
    echo "Catching node up to ${LATEST_CATCHPOINT} ..."
    goal node catchup ${LATEST_CATCHPOINT}
  else
    echo "Node actively catching up with latest block ..."
  fi
}

echo "Waiting for algod ..."
timeout 22 sh -c 'until nc -z $0 $1; do sleep 1; done' localhost ${ALGORAND_CLAIMER_ALGOD_PORT}

# catchup the algod node to a recent block
echo "Catching up algod node ..."
counter=0
max_retry=10
sleep_seconds=1
while ! catchup; do
  sleep $sleep_seconds
  [[ counter -eq $max_retry ]] && echo "Failed to catchup node, exiting ..." && exit 0
  echo "Failure to catchup node - $(($max_retry-$counter)) attempts left ..."
  ((counter++))
done
echo "Finished catchup ..."
sleep infinity
