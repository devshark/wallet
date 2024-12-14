# contains all the source files
FROM golang:1.22 AS base 

COPY ./ /app

WORKDIR /app

RUN make vendor && make short-test

# will only build the binary
# source files + binary
FROM base AS builder

RUN make build

# we only need the smallest image possible for prod image
FROM alpine:3.21 AS prod

RUN apk update && apk upgrade && apk add --no-cache ca-certificates

WORKDIR /app

# run as regular, non-root user
RUN addgroup -S app && adduser -S app -G app
USER app

COPY --from=builder /app/build/ /app/
COPY --from=builder /app/migrations/ /app/migrations/

CMD [ "/app/http" ]