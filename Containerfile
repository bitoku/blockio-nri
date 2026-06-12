FROM golang:1.25 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o 90-nri-iops .

FROM registry.access.redhat.com/ubi9-micro:latest

COPY --from=builder /src/90-nri-iops /opt/nri/plugins/90-nri-iops
COPY examples/nri-iops.conf /etc/nri/conf.d/90-nri-iops.conf

ENTRYPOINT ["/opt/nri/plugins/90-nri-iops"]
