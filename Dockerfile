FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o /bin/app ./cmd/main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=build /bin/app /app
COPY config.toml /

ARG PG_DSN
ARG APP_STORE_API_HOST

ENV PG_DSN=$PG_DSN
ENV APP_STORE_API_HOST=$APP_STORE_API_HOST

USER nonroot

ENTRYPOINT ["/app"]
