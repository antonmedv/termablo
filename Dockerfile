FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /termablo . && mkdir /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /termablo /termablo
COPY --from=build --chown=nonroot:nonroot /data /data
VOLUME /data
EXPOSE 2222
ENTRYPOINT ["/termablo", "-ssh", ":2222", "-hostkey", "/data/host_ed25519"]
