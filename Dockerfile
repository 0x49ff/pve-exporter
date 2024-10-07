FROM golang:1.22.8 as build

WORKDIR /build
COPY . .
RUN go mod init proxmox-ve-exporter; go mod tidy
RUN GOOS=linux CGO_ENABLED=0 go build ./main.go

FROM alpine:latest
WORKDIR /app
COPY --from=build /build/main /app/main
CMD ["/app/main"]