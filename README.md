# Nanit Stream Proxy

This is sleepless night induced pet project to restream Nanit Baby Monitor live stream for local viewing.

## TL;DR

```bash
docker run --rm \
  -e NANIT_EMAIL=your@email.tld \
  -e NANIT_PASSWORD=XXXXXXXXXXXXX \
  -p 8080:8080 \
  registry.gitlab.com/adam.stanek/nanit:v0-2
```

Open http://127.0.0.1:8080 in Safari

## Features

- Authenticates to Nanit servers
- Retrieves RTMP stream
- Exposes the stream as HLS so that it can be directly AirPlayed to the TV

## Why?

- I wanted to learn something new on paternity leave (first project in Go!)
- Nanit iOS application is nice, but I was really disappointed that it cannot properly stream to TV through AirPlay. As anxious parents of our first child we wanted to have it playing in the background on TV when we are in the kitchen, etc. When AirPlaying it from the phone it was really hard to see the little one in portrait mode + the sound was crazy quiet. This helps us around the issue and we don't have to drain our phone batteries.

## Usage

Application is ready to be used in Docker. You can use environment variables for configuration. For more info see [.env.sample](.env.sample).

## How to develop

```bash
go mod download
go run src/*.go
```

## Disclaimer

I made this program solely for learning purposes. Please use it at your own risk and always follow any terms and conditions which might be applied when communicating to Nanit servers.

This program is free software. It comes without any warranty, to
the extent permitted by applicable law. You can redistribute it
and/or modify it under the terms of the Do What The Fuck You Want
To Public License, Version 2, as published by Sam Hocevar. See
http://sam.zoy.org/wtfpl/COPYING for more details.