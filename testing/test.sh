#!/bin/bash

cat <<EOF
This is a simple script to run the import in a throaway docker instance 
of Home Assistant.

In this way you can preview the result before committing to change your 
Home Assistant configuration.

To run it you need to export your ESB information first.

export esb_password=...
export esb_user=...
export mprn=...

EOF

version=latest

checkVar(){
    if [ -z "$2" ]; then 
        echo >&2 "ERROR: missing export $1"
        exit 1
    fi
}

title(){
    echo
    echo "** $1"
    echo "============================================"
}

checkVar "esb_password=your_password" "$esb_password"
checkVar "esb_user=your_username" "$esb_user"
checkVar "mprn=your_mprn" "$mprn"

title "Building docker images"
(cd .. && docker build --tag=esb2ha:$version . )
docker build --tag=esb2ha_test_ha:$version .

network=esb2ha_testing
ha_name=testingHomeAssistant

function cleanup() {
    title "Stopping testing Home Assistant instance"
    docker kill "$ha_name"
    title "Removing docker network"
    docker network rm "$network"
    title "Done"
}
trap cleanup EXIT

title "Creating docker network"
docker network create "$network"

title "Starting testing Home Assistant instance"
docker run --network="$network" --name "$ha_name" -d --rm -p 127.0.0.1:8123:8123 esb2ha_test_ha:$version

# Give some time to Home Assistant to start
sleep 2s

ha_token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJjNjQ2YTU4NzM3M2U0NDVhYTU1NDJkYWI4ZTRiMGEwYyIsImlhdCI6MTY4OTk2NjQ0MiwiZXhwIjoyMDA1MzI2NDQyfQ.nBlvEFpqvKWkpWK5WGv1x-fOSpRMYO7RYmEhAcW0_Og

title "Running esb2ha"
docker run --network="$network" --rm esb2ha:$version pipe \
    --ha_server="$ha_name:8123" \
    --ha_sensor="sensor.esb_electricity_usage" \
    --ha_token="$ha_token" \
    --esb_password="$esb_password" \
    --esb_user="$esb_user" \
    --mprn="$mprn"

cat <<EOF

If you didn't see any error, you can check your data on
http://localhost:8123/energy

1. log in with username 'test' and password 'test'
2. click "add consumption"
3. select the sensor "ESB electricity usage"
4. save and click "next" a few times

After clicking "Show me my energy dashboard" remember that 
today's data is not available, so you need to change day
or change to weekly or monthly view.

Press enter to stop the testing environment.
EOF

read
