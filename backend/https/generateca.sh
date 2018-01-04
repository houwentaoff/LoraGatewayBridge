#!/bin/bash - 
#===============================================================================
#
#          FILE: generateca.sh
# 
#         USAGE: ./generateca.sh 
# 
#   DESCRIPTION: 
# 
#       OPTIONS: ---
#  REQUIREMENTS: ---
#          BUGS: ---
#         NOTES: ---
#        AUTHOR: Dr. Fritz Mehner (fgm), mehner.fritz@fh-swf.de
#  ORGANIZATION: FH Südwestfalen, Iserlohn, Germany
#       CREATED: 01/02/2018 09:51
#      REVISION:  ---
#===============================================================================

set -o nounset                              # Treat unset variables as an error
openssl genrsa -out server.key 2048

#ca
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -subj "/CN=bwcpn" -days 5000 -out ca.crt

openssl req -new -key server.key -subj "/CN=bwcpn" -out server.csr
#openssl req -new -x509 -key server.key -subj "/CN=bwcpn" -out server.crt -days 365
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 5000

# client
openssl genrsa -out client.key 2048
openssl req -new -key client.key -subj "/CN=bwcpn" -out client.csr
#openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out 
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -extfile client.ext -out client.crt -days 5000
