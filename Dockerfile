### Build stage
FROM golang:1.23 AS build

WORKDIR /src

# Speed up builds with module caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the API server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api ./cmd/api


### Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=build /out/api /api

# Cloud Run sets PORT; our config defaults to ":" + PORT when HTTP_ADDR is empty.
ENV APP_ENV=prod

USER nonroot:nonroot

ENTRYPOINT ["/api"]


