# Build a tiny static binary, then ship it in a minimal image.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/touchline ./cmd/touchline

FROM alpine:3.20
# /data persists the SSH host key across deploys so the fingerprint is stable.
RUN adduser -D -h /home/touchline touchline \
    && mkdir -p /data \
    && chown touchline /data
WORKDIR /home/touchline
COPY --from=build /out/touchline /usr/local/bin/touchline
COPY --from=build /src/mock ./mock
USER touchline
ENV TOUCHLINE_SSH=true \
    TOUCHLINE_SSH_ADDR=0.0.0.0:23234 \
    TOUCHLINE_SSH_HOST_KEY_PATH=/data/touchline_ed25519
EXPOSE 23234
ENTRYPOINT ["touchline"]
