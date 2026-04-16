FROM scratch

# this pulls directly from the upstream image, which already has ca-certificates:
COPY --from=alpine:latest@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY metrics-actioner /
ENTRYPOINT ["/metrics-actioner"]
