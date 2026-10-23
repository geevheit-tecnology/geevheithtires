FROM golang:1.26.10-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server && CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate
FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D app
WORKDIR /app
COPY --from=build /out/ /app/
COPY --chmod=755 start-render.sh /app/start-render.sh
USER app
EXPOSE 8080
CMD ["/app/server"]
