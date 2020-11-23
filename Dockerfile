FROM golang:1.14-stretch AS build
ADD src/app /go/src/app
WORKDIR /go/src
RUN go build -o /go/bin/nanit app/*.go

FROM debian:stretch
RUN apt-get -yqq update && \
    apt-get install -yq --no-install-recommends ca-certificates ffmpeg && \
    apt-get autoremove -y && \
    apt-get clean -y
RUN mkdir -p /app/data
COPY --from=build /go/bin/nanit /app/nanit
ADD ./src/static /app/static
WORKDIR /app
CMD ["/app/nanit"]