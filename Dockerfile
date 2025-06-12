FROM golang:1.21 AS builder

WORKDIR /cel-gun

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o cel-gun .

FROM alpine:latest

WORKDIR /cel-gun

# Copy the binary from the builder
COPY --from=builder /cel-gun/cel-gun .

# Set environment variable defaults (can be overridden at runtime)
ENV GUN_NETWORK="mocha-4" \
    GUN_TARGET="/dnsaddr/da-bootstrapper-1-mocha-4.celestia-mocha.com/p2p/12D3KooWCBAbQbJSpCpCGKzqz3rAN4ixYbc63K68zJg9aisuAajg" \
    GUN_NAMESPACE="00000000000000000000000000000000000000000000004365726f41" \
    GUN_HEIGHT=6490556

# Entry point
ENTRYPOINT ["./cel-gun"]
