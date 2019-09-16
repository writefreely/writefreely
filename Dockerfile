FROM golang:1.12-alpine AS build

RUN apk add nodejs nodejs-npm make g++ ca-certificates git sqlite-dev && \
  npm install -g less less-plugin-clean-css && \
  go get -u github.com/jteeuwen/go-bindata/...

WORKDIR /src
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY . .
RUN cd cmd/writefreely && go build -v -tags='sqlite'
RUN make assets ui

RUN mkdir -p \
  /home/writefreely/static /home/writefreely/templates /home/writefreely/pages && \
  cp -r templates/ pages/ static/ /home/writefreely && \
  cp config.ini.example /home/writefreely/config.ini

FROM alpine AS final

# TODO user nobody or similar
COPY --from=build /src/cmd/writefreely/writefreely /bin
COPY --from=build /home /home

EXPOSE 8080
WORKDIR /home/writefreely
ENTRYPOINT [ "writefreely" ]
