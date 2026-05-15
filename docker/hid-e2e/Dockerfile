FROM golang:1.26-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bluez \
        bluez-test-tools \
        dbus \
        kmod \
        procps \
    && rm -rf /var/lib/apt/lists/*

COPY docker/hid-e2e/run.sh /usr/local/bin/hid-e2e
RUN chmod +x /usr/local/bin/hid-e2e

ENTRYPOINT ["/usr/local/bin/hid-e2e"]
