FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/ledger ./cmd/ledger

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S ledger \
    && adduser -S -G ledger ledger

COPY --from=build /out/ledger /usr/local/bin/ledger

USER ledger
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/ledger"]
