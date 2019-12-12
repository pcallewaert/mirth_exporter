FROM golang:1.13-alpine as builder

# Install Git + UPX.
RUN apk add --no-cache git

# Set the Current Working Directory inside the container
WORKDIR $GOPATH/src/github.com/pcallewaert/mirth_exporter

# Copy everything from the current directory inside the container
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux \
 go build -a -ldflags '-w -extldflags "-static"' -o /tmp/mirth_exporter -mod=vendor

# Production container
FROM openjdk:8-alpine

RUN apk update && \
 apk add --no-cache ca-certificates tzdata && \
 update-ca-certificates && \
 addgroup --system app && adduser -S -G app app

WORKDIR /home/app

COPY --from=builder /tmp/mirth_exporter /home/app/mirth_exporter
COPY scripts scripts


# Use the unprivileged user.
USER app

# Run the binary.
ENTRYPOINT ["/home/app/scripts/entrypoint.sh"]