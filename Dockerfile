# builder image
FROM golang:1.13-alpine3.11 as builder
RUN mkdir /build
ADD *.go go.mod go.sum /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o update-dynamic-ip .

FROM alpine:3.11.3
COPY --from=builder /build/update-dynamic-ip .
CMD [ "./update-dynamic-ip" ]
