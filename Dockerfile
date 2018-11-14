FROM golang:1.11.2-alpine3.8

RUN apk add --update nodejs nodejs-npm make git
RUN npm install -g less
RUN npm install -g less-plugin-clean-css

WORKDIR /go/src/app
COPY . .

RUN make install
RUN make ui
RUN make deps

EXPOSE 8080
CMD ["writefreely"]
