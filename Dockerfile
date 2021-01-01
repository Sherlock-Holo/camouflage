FROM golang:alpine AS build-env

RUN apk add --no-cache git

ADD . /src

WORKDIR /src

# proxy
ENV GOPROXY="https://goproxy.cn|direct"

RUN go mod download

ENV CGO_ENABLED=0

RUN go build -v

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=build-env /src/camouflage /camouflage

ENTRYPOINT [ "/camouflage" ]

CMD [ "client", "-f", "/conf.toml" ]
