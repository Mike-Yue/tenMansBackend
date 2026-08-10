# ---- build stage ----
FROM golang:1.26 AS build
WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

# Build a static, pure-Go binary. modernc.org/sqlite needs no CGO, so the
# resulting binary runs on a minimal scratch/distroless image. The embedded
# seed database (db/seed.db) is baked into the binary here.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

# ---- run stage ----
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/server /server
# Render overrides the listen port via $PORT; 8080 is only the local default.
EXPOSE 8080
ENTRYPOINT ["/server"]
