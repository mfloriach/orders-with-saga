FROM golang:1.27-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/api
RUN CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/worker /app/worker
COPY --from=build /app/api/openapi.yaml /app/api/openapi.yaml
EXPOSE 8080
ENTRYPOINT ["/app/server"]
