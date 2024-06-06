FROM golang:1.22-alpine AS builder

# Create appuser
ENV USER=hpascaler
ENV UID=21948

# See https://stackoverflow.com/a/55757473/12429735RUN
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

# Copy to the container
WORKDIR /app
COPY . /app

# Fetch dependencies
RUN go mod download
RUN go mod verify

# Build the binary
ENV CGO_ENABLED=0
RUN go build -ldflags="-w -s" -o hpa-time-scaler

# ------------- Runtime Stage -------------
FROM scratch

# Copy User from Builder
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy executable
COPY --from=builder /app/hpa-time-scaler /usr/local/bin/

# Run as unprivileged user
USER hpascaler:hpascaler

ENTRYPOINT ["hpa-time-scaler"]
