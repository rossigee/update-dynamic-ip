# builder image
FROM golang:1.18.2-alpine3.15 as builder
RUN mkdir /build
ADD *.go go.mod go.sum /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o update-dynamic-ip .

FROM scratch
COPY --from=builder /build/update-dynamic-ip .
USER 65535
CMD [ "./update-dynamic-ip" ]
