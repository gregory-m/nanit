ARG BASE_IMAGE_TAG

FROM golang:1.15.5-buster AS build
ADD src /app/src
ADD go.mod /app/
ADD go.sum /app/
WORKDIR /app
ARG CI_COMMIT_SHORT_SHA
RUN go build -ldflags "-X main.GitCommit=$CI_COMMIT_SHORT_SHA" -o ./bin/nanit ./src/*.go

FROM registry.gitlab.com/adam.stanek/nanit/base:$BASE_IMAGE_TAG
RUN mkdir -p /app/data
COPY --from=build /app/bin/nanit /app/bin/nanit
WORKDIR /app
CMD ["/app/bin/nanit"]