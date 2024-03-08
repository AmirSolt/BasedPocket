FROM alpine:latest


RUN apk add --no-cache \
    ca-certificates


# build
CMD ["go", "build", '-ldflags "-X main.CGO_ENABLED=0"']

EXPOSE 8080
# start PocketBase
CMD ["./basedpocket", "serve", "--http=0.0.0.0:8080"]