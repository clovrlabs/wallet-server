FROM golang:1.19-alpine as builder

WORKDIR /build

ADD . . 

RUN go get .

RUN go build -o server . 


FROM alpine:3

COPY --from=builder  /build/server /server
COPY --from=builder  /build/postgresql/migrations /migrations

RUN chmod +x /server

ENTRYPOINT [ "/server" ]