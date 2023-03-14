#!/bin/bash

go build -o client client.go

N=${1:-1}
echo "N=${N}"

for (( i=1; i<=${N}; i++ ))
do
  ./client &
  echo "spawned client ${i}"
done

wait
kill -- -$$
