# --- build: static, CGO-free binary ---
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/scanner ./cmd/scanner

# --- runtime: distroless, nonroot ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/scanner /usr/local/bin/scanner
# seed config (mount writable copies in production; users.jsonl is written by admin CRUD)
COPY studies.jsonl users.jsonl ./
EXPOSE 8080
ENTRYPOINT ["scanner"]
CMD ["serve"]
