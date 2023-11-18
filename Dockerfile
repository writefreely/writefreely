# Build image
# SHA256 of golang:1.21-alpine3.18 linux/amd64
FROM golang@sha256:f475434ea2047a83e9ba02a1da8efc250fa6b2ed0e9e8e4eb8c5322ea6997795 as build

LABEL org.opencontainers.image.source="https://github.com/writefreely/writefreely"
LABEL org.opencontainers.image.description="WriteFreely is a clean, minimalist publishing platform made for writers. Start a blog, share knowledge within your organization, or build a community around the shared act of writing."

RUN apk -U upgrade \
    && apk add --no-cache nodejs npm make g++ git \
    && npm install -g less less-plugin-clean-css \
    && mkdir -p /go/src/github.com/writefreely/writefreely

WORKDIR /go/src/github.com/writefreely/writefreely

COPY . .

RUN cat ossl_legacy.cnf > /etc/ssl/openssl.cnf

ENV GO111MODULE=on
ENV NODE_OPTIONS=--openssl-legacy-provider

RUN make build \
    && make ui \
    && mkdir /stage \
    && cp -R /go/bin \
      /go/src/github.com/writefreely/writefreely/templates \
      /go/src/github.com/writefreely/writefreely/static \
      /go/src/github.com/writefreely/writefreely/pages \
      /go/src/github.com/writefreely/writefreely/keys \
      /go/src/github.com/writefreely/writefreely/cmd \
      /stage

# Final image
# SHA256 of alpine:3.18.4 linux/amd64
FROM alpine@sha256:48d9183eb12a05c99bcc0bf44a003607b8e941e1d4f41f9ad12bdcc4b5672f86

RUN apk -U upgrade \
    && apk add --no-cache openssl ca-certificates

COPY --from=build --chown=daemon:daemon /stage /go

WORKDIR /go
VOLUME /go/keys
EXPOSE 8080
USER daemon

ENTRYPOINT ["cmd/writefreely/writefreely"]

HEALTHCHECK --start-period=5s --interval=15s --timeout=5s \
    CMD curl -fSs http://localhost:8080/ || exit 1