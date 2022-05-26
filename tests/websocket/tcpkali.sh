#!/bin/bash

tcpkali --ws -c 100 -m 'hello, sonic!' -r 10k 127.0.0.1:8080
