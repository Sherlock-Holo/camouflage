# camouflage
a mux websocket over TLS proxy

[![Build Status](https://travis-ci.org/Sherlock-Holo/camouflage.svg?branch=master)](https://travis-ci.org/Sherlock-Holo/camouflage)

## Feature
- standard websocket over TLS
- mux on websocket
- verify client by TOTP

## Usage
1. prepare your server tls key and crt file
2. use `camouflage genTOTPSecret` to generate TOTP secret and period
3. prepare a static web root (optional)
4. write config correctly
5. enjoy it

```
camouflage is a mux websocket over TLS proxy

Usage:
  camouflage [command]

Available Commands:
  client        client mode
  genTOTPSecret generate TOTP secret, default period is 60
  help          Help about any command
  server        server mode

Flags:
  -h, --help      help for camouflage
      --version   version for camouflage

Use "camouflage [command] --help" for more information about a command.
```

## Notice
you can use self_sign ca for camouflage client to auth self_sign server certificate, however I don't recommend to do it

## Advanced usage
use cf or some thing else can proxy websocket over TLS to optimize speed