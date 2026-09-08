FROM golang:1.25 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/bot ./cmd/bot
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/payment-refund ./cmd/payment-refund

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/bot /app/bot
COPY --from=builder /out/payment-refund /app/payment-refund
COPY migrations /app/migrations
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/bot"]
