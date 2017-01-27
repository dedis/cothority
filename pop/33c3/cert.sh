#!/usr/bin/env bash
openssl genrsa -out server.key 2048
echo -e "\n\n\n\n\n\n\n" | \
	openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650
echo
echo "Done"