FROM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/server .

FROM alpine:3.21

RUN adduser -D -u 10001 app
USER app

COPY --from=builder /out/server /usr/local/bin/server

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
