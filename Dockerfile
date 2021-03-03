# Build image
FROM golang:1.14-alpine as build

RUN apk add --update nodejs nodejs-npm make g++ git
RUN npm install -g less less-plugin-clean-css
RUN go get -u github.com/go-bindata/go-bindata/...

RUN mkdir -p /go/src/github.com/writeas/writefreely
WORKDIR /go/src/github.com/writeas/writefreely

COPY . .

ENV GO111MODULE=on

RUN make build \
  && make ui
RUN mkdir /stage && \
    cp -R /go/bin \
      /go/src/github.com/writeas/writefreely/templates \
      /go/src/github.com/writeas/writefreely/static \
      /go/src/github.com/writeas/writefreely/pages \
      /go/src/github.com/writeas/writefreely/keys \
      /go/src/github.com/writeas/writefreely/cmd \
      /stage

# Final image
FROM alpine:3.12

RUN apk add --no-cache openssl ca-certificates
COPY --from=build --chown=daemon:daemon /stage /go

WORKDIR /go
VOLUME /go/keys
EXPOSE 8080
USER daemon

ENTRYPOINT ["cmd/writefreely/writefreely"]
