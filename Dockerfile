FROM chromedp/headless-shell:latest

RUN apt-get update

RUN apt-get install -y --no-install-recommends \
    dumb-init \
    git \
    ca-certificates \
    curl \
    build-essential

RUN curl https://dl.google.com/go/go1.16.4.linux-amd64.tar.gz > /tmp/go1.16.4.linux-amd64.tar.gz && \
    rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go1.16.4.linux-amd64.tar.gz


WORKDIR /home/chrome


ENV PATH=${PATH}:/usr/local/go/bin
RUN go version

COPY go.* ./
RUN go mod download
RUN cat go.mod

COPY *.go .
RUN GOOS=linux CGO_ENABLED=1 GOARCH=amd64 go build -a -o . ./

# Add user so we don't need --no-sandbox in Chromium
RUN groupadd chrome && useradd -g chrome -s /bin/bash -G audio,video chrome \
    && mkdir -p /home/chrome/Downloads \
    && chown -R chrome:chrome /home/chrome

# Run everything after as non-privileged user.
USER chrome

COPY start.sh .
ENTRYPOINT ["dumb-init", "--", "./start.sh"]
