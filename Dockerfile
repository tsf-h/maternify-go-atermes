# ------- BUILD STAGE -------
FROM golang:1.25-rc-bullseye AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -o server main.go


FROM --platform=linux/arm64 mcr.microsoft.com/playwright:v1.52.0-jammy

RUN apt-get update && apt-get install -y wget && \
    wget https://go.dev/dl/go1.23.3.linux-arm64.tar.gz && \
    tar -C /usr/local -xzf go1.23.3.linux-arm64.tar.gz && \
    rm go1.23.3.linux-arm64.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app

COPY --from=builder /app/server .
COPY go.mod go.sum ./

RUN PLAYWRIGHT_VERSION=$(grep 'github.com/playwright-community/playwright-go' go.mod | awk '{print $2}') && \
    go run github.com/playwright-community/playwright-go/cmd/playwright@${PLAYWRIGHT_VERSION} install --with-deps chromium

EXPOSE 8080 6060

CMD ["./server"]