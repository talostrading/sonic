#!/bin/bash


rates=(10 100 1000 10000)
conns=(1 2 4 8 16 32 64 96 128 160 192 224 256)

function run() {
    which="$1"
    echo "running test-suite for ${which}"

    for rate in "${rates[@]}"; do
        echo "starting server at rate=${rate} for ${which}"
        ./server/target/release/server "${rate}" > "server_${which}_${rate}Hz.log" &
        server_pid=$!

        for conn in "${conns[@]}"; do
            echo "running ${which} tests rate=${rate} conn=${conn}"
            go run client-"${which}"/main.go -n "${conn}" -rate "${rate}"
            echo "done"
            sleep 5
        done

        kill $server_pid
        echo "done with tests at rate=${rate} for ${which}"

        sleep 1
    done

    sleep 2
}

run "std"
echo "--------------------------------"
run "sonic"

echo "bye :)"
