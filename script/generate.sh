rm -rf ca server client
mkdir ca server client

openssl genrsa -out ca/ca.key 4096
openssl req -x509 -new -nodes -key ca/ca.key -subj "/C=CN" -days 3650 -out ca/ca.crt

openssl genrsa -out server/server.key 4096
openssl req -new -key server/server.key \
        -subj "/C=CN/CN=$1" -sha256 \
        -out server/server.csr

openssl x509 -req -days 3650 -in server/server.csr -CA ca/ca.crt -CAkey ca/ca.key -CAcreateserial \
        -sha256 \
        -out server/server.crt -extfile server.cnf -extensions SAN

openssl genrsa -out client/client.key 4096
openssl req -new -key client/client.key -subj "/CN=camouflage" -out client/client.csr
openssl x509 -req -days 3650 -in client/client.csr -CA ca/ca.crt -CAkey ca/ca.key -CAcreateserial \
        -extfile client.cnf \
        -sha256 \
        -out client/client.crt 
