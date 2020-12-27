# Nanit Stream Proxy

This is sleepless night induced pet project to restream Nanit Baby Monitor live stream for local viewing.

## Features

- Restreaming of live feed
  - Over HLS or to your local RTMP server
  - Both local and remote streaming supported
  - Option to run custom stream processor (like FFMPEG)
- Retrieving sensors data from cam (temperature and humidity) and publishing them over MQTT
- Graceful authentication session handling
- Works as a companion for your Home-assistant / Homebridge setup (see [guides](#setup-guides) below)

## TL;DR

### a) Restream live feed as HLS

```bash
docker run --rm \
  -e NANIT_EMAIL=your@email.tld \
  -e NANIT_PASSWORD=XXXXXXXXXXXXX \
  -p 8080:8080 \
  registry.gitlab.com/adam.stanek/nanit:v0-4
```

Open http://127.0.0.1:8080 in Safari

### b) Restream to local RTMP server

```yaml
version: '2.1'
services:
  nanit:
    image: registry.gitlab.com/adam.stanek/nanit:v0-4
    environment:
    - NANIT_EMAIL=your@email.tld
    - NANIT_PASSWORD=XXXXXXXXXXXXX
    - NANIT_HLS_ENABLED=false
    - NANIT_LOCAL_STREAM_ENABLED=true
    - NANIT_LOCAL_STREAM_PUSH_TARGET=rtmp://{your_ip_not_localhost}:1935/live
  nginx:
    image: tiangolo/nginx-rtmp
    ports:
    - '1935:1935'
```

Open `rtmp://127.0.0.1:1935/live` in VLC

**Notice:** The cam does not seem to react well if it tries to start streaming at the time when RTMP server is not ready. It is advised to ensure nginx-rtmp is started first. If the streaming does not start in few seconds, try to restart the nanit container to reinitiate the streaming.

### Setup guides

- [Home assistant](./docs/home-assistant.md)
- [Homebridge](./docs/homebridge.md)
- [Sensors](./docs/sensors.md)

### Further usage

Application is ready to be used in Docker. You can use environment variables for configuration. For more info see [.env.sample](.env.sample).

### HLS vs Local RTMP

**HLS:**

- (+) HLS is viewable from Safari browser, from there you can directly AirPlay the video to your TV
- (-) HLS is not designed to by low-latency. The stream is split into chunks of certain size which are then downloaded by clients. Because of that you are always behind at least by the size of a chunk + network delay.
- (-) HLS is not easily consumable by Home Assistant / Homebridge part

**RTMP:**

- (+) You can reuse the stream served RTMP server to not care about any authentication on Home Assistant / Homebridge part
- (+) It is streamed directly from the cam, does not go through Nanit servers
- (-) You need additional RTMP server to relay the content to clients
- (-) RTMP is not openable in any web browser
- **(-) We cannot handle any dropouts. Cam is communicating directly with that server. There is no way of knowing if the streaming stopped for some reason. Or if it was even initiated properly.**

## Why?

- I wanted to learn something new on paternity leave (first project in Go!)
- Nanit iOS application is nice, but I was really disappointed that it cannot properly stream to TV through AirPlay. As anxious parents of our first child we wanted to have it playing in the background on TV when we are in the kitchen, etc. When AirPlaying it from the phone it was really hard to see the little one in portrait mode + the sound was crazy quiet. This helps us around the issue and we don't have to drain our phone batteries.

## How to develop

```bash
go run cmd/nanit/*.go

# On proto file change
protoc --go_out . --go_opt=paths=source_relative pkg/client/websocket.proto

# Run tests
go test ./pkg/...
```

For some insights see [Developer notes](docs/developer-notes.md).

## Disclaimer

I made this program solely for learning purposes. Please use it at your own risk and always follow any terms and conditions which might be applied when communicating to Nanit servers.

This program is free software. It comes without any warranty, to
the extent permitted by applicable law. You can redistribute it
and/or modify it under the terms of the Do What The Fuck You Want
To Public License, Version 2, as published by Sam Hocevar. See
http://sam.zoy.org/wtfpl/COPYING for more details.