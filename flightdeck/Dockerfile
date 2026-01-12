ARG GO_VERSION=1.25
# build golang
FROM golang:${GO_VERSION}-alpine AS go-stage
WORKDIR /build
# Copy team-operator module files
COPY --from=root go.mod go.sum ./
COPY --from=root api ./api
COPY --from=root internal ./internal
RUN go mod download
# Copy flightdeck source
COPY . ./flightdeck
ARG VERSION
WORKDIR /build/flightdeck
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X 'github.com/posit-dev/team-operator/flightdeck/internal.VersionString=${VERSION}'"\
    -a \
    -o flightdeck \
    ./cmd/

# production image
FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.source=https://github.com/posit-dev/team-operator
WORKDIR /app
COPY --from=go-stage /build/flightdeck/flightdeck /app/flightdeck
COPY --from=go-stage /build/flightdeck/public /app/public
ENV PTD_PORT=8080 PTD_RELATIVE_STATIC=false
USER 999:999
EXPOSE 8080
CMD ["/app/flightdeck"]
