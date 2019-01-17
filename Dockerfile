FROM golang:1.11-alpine as builder

# Install:
# - glide and git for dependencies management
# - shadow to create unprivileged user
RUN apk add glide git shadow

# Prepare directory for source code and empty directory, which we copy
# to scratch image
RUN mkdir -p /go/src/validating-admission-webhook-server/tmp

# Copy glide files first and install dependencies to cache this layer
ADD ./glide.lock ./glide.yaml /go/src/validating-admission-webhook-server/
WORKDIR /go/src/validating-admission-webhook-server
RUN glide install

# Add source code
ADD . /go/src/validating-admission-webhook-server

# Build binary without linking to glibc, so we can use scratch image
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o validating-admission-webhook-server .

FROM scratch
# Copy executable
COPY --from=builder /go/src/validating-admission-webhook-server/tmp /validating-admission-webhook-server
COPY --from=builder /go/src/validating-admission-webhook-server/validating-admission-webhook-server /validating-admission-webhook-server/validating-admission-webhook-server
# Required for running as nobody
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
# Required for writing logs
COPY --from=builder --chown=nobody:nobody /go/src/validating-admission-webhook-server/tmp /tmp
USER nobody
WORKDIR /validating-admission-webhook-server
ENTRYPOINT ["./validating-admission-webhook-server"]
