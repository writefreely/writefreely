FROM golang:1.11.2-alpine3.8 as build

RUN apk add --update nodejs nodejs-npm make git
RUN npm install -g less
RUN npm install -g less-plugin-clean-css

WORKDIR /go/src/app
COPY . .

RUN make install
RUN make ui
RUN make deps

RUN mkdir /stage && \
    cp -R /go/bin \
       /go/src/app/templates \
       /go/src/app/static \
       /go/src/app/schema.sql \
       /go/src/app/pages \
       /go/src/app/keys \
      /stage && ls -l /stage

FROM golang:1.11.2-alpine3.8

COPY --from=build --chown=daemon:daemon /stage /go

VOLUME /go/keys
EXPOSE 8080
USER daemon

CMD ["writefreely"]
