FROM golang:1.14-stretch AS build
ADD src /app/src
ADD go.mod /app/
ADD go.sum /app/
WORKDIR /app
RUN go build -o ./bin/nanit ./src/*.go

FROM debian:stretch
RUN apt-get -yqq update && \
    apt-get install -yq --no-install-recommends ca-certificates ffmpeg && \
    apt-get autoremove -y && \
    apt-get clean -y
RUN mkdir -p /app/data
COPY --from=build /app/bin/nanit /app/bin/nanit
WORKDIR /app
CMD ["/app/bin/nanit"]