FROM chromedp/headless-shell:latest


RUN apt-get update && apt-get install -y --no-install-recommends \
    dumb-init \
    golang


# Add user so we don't need --no-sandbox in Chromium
RUN groupadd chrome && useradd -g chrome -s /bin/bash -G audio,video chrome \
    && mkdir -p /home/chrome/Downloads \
    && chown -R chrome:chrome /home/chrome

WORKDIR /home/chrome

COPY go.* ./
RUN go mod download
RUN cat go.mod

COPY . .
RUN GOOS=linux CGO_ENABLED=1 GOARCH=amd64 go build -a -o /app ./

# Run everything after as non-privileged user.
USER chrome


ENTRYPOINT ["dumb-init", "--"]
# CMD ["/path/to/your/program"]