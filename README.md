# camouflage
a mux websocket over TLS proxy

[![Build Status](https://travis-ci.org/Sherlock-Holo/camouflage.svg?branch=master)](https://travis-ci.org/Sherlock-Holo/camouflage)

## Usage
1. prepare your server tls key and crt file
2. generate your ca and client tls key and crt file (recommend use gnutls)
3. prepare a static web root (optional)
4. write config correctly
5. enjoy it

## Notice
you can use self_sign ca for camouflage client to auth self_sign server certificate, however I don't recommend to do it