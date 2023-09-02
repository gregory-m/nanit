FROM golang:1.21 AS build
ADD . /app/
WORKDIR /app
RUN CGO_ENABLED=0 go build -ldflags "-X main.GitCommit=$(git rev-parse --short HEAD)" -o ./bin/nanit ./cmd/nanit/*.go

FROM alpine
RUN mkdir -p /app/data
COPY --from=build /app/bin/nanit /app/bin/nanit
WORKDIR /app
VOLUME [ "/app/data" ]
CMD ["/app/bin/nanit"]