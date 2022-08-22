#!/bin/bash

echo "$1 connections"

tcpkali --connections $1 --connect-rate $1 --nagle=off --duration 10 --workers 4 --latency-connect -m "1234567890abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqrstuvwxyz" 127.0.0.1:8080
