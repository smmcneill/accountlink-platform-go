FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY build/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]
