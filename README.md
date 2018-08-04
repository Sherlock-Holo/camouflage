# camouflage
a mux websocket+TLS proxy

[![Build Status](https://travis-ci.org/Sherlock-Holo/camouflage.svg?branch=master)](https://travis-ci.org/Sherlock-Holo/camouflage)

## Usage
1. edit script/server.cnf, edit the last 2 lines if you don't know how to custom it.
2. run `generate.sh` , for example `./generate.sh 127.0.0.1` .
3. copy `config/config.conf` to what you want and edit it to fit what you generated.
4. run `camouflage`

## Windows support
because of go problems, in windows system I can't use `x509.SystemCertPool()`, so if you want to compile windows version, you can download 
> https://gist.github.com/Sherlock-Holo/802a67a2beb9de480aef39bbf2953c8f

and rename as `rootCA_windows.go` and then put it in `camouflage/ca` directory. So you can compile on windows now.

## Notice
If you want to use camouflage, please make sure the dep [link](https://github.com/Sherlock-Holo/link) is the least version to enjoy the fast speed and good stability.