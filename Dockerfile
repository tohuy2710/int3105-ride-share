FROM golang:1.25.9-alpine@sha256:5caaf1cca9dc351e13deafbc3879fd4754801acba8653fa9540cea125d01a71f AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=ride
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/server ./cmd/${SERVICE}

FROM alpine:3.23@sha256:ff7f0c727dd796aa36787bfcd9a64bf35a6ce72a73451b3359fde849b6703d63
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/server /app/server
COPY fixtures /app/fixtures
USER app
ENV GIN_MODE=release LOG_LEVEL=info
ENTRYPOINT ["/app/server"]
