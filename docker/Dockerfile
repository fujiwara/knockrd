FROM alpine:3.11
RUN apk --no-cache add ca-certificates
COPY dist/knockrd_linux_amd64/knockrd /usr/local/bin/knockrd
COPY docker/config.yaml /etc/knockrd/config.yaml
ENV KNOCKRD_CONFIG /etc/knockrd/config.yaml
ENTRYPOINT ["/usr/local/bin/knockrd"]
