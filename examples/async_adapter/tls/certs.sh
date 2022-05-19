#!/bin/bash

mkdir -p certs
rm certs/*

echo "server cert"
openssl req -new -nodes -x509 -out certs/server.pem -keyout certs/server.key -days 3650 -subj "/C=DE/ST=NRW/L=Earth/O=Random Company/OU=IT/CN=www.random.com/emailAddress=sergiu@talos.com"

echo "client cert"
openssl req -new -nodes -x509 -out certs/client.pem -keyout certs/client.key -days 3650 -subj "/C=DE/ST=NRW/L=Earth/O=Random Company/OU=IT/CN=www.random.com/emailAddress=sergiu@talos.com"

