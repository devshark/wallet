FROM golang:1.22 as builder

COPY ./ /app

WORKDIR /app

RUN make vendor && make short-test && make build

FROM alpine:3.21

RUN apk update && apk upgrade && apk add --no-cache ca-certificates

WORKDIR /app

# run as regular, non-root user
RUN addgroup -S app && adduser -S app -G app
USER app

COPY --from=builder /app/build/ /app/

CMD [ "/app/http" ]