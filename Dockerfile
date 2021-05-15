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


# X11, sound
RUN apt-get install -y --no-install-recommends \
    gconf-service \
    libasound2 \
    libatk1.0-0 \
    libatk-bridge2.0-0 \
    libc6 \
    libcairo2 \
    libcups2 \
    libdbus-1-3 \
    libexpat1 \
    libfontconfig1 \
    libgcc1 \
    libgconf-2-4 \
    libgdk-pixbuf2.0-0 \
	libglib2.0-0 \
	libgtk-3-0 \
	libnspr4 \
	libpango-1.0-0 \
	libpangocairo-1.0-0 \
	libstdc++6 \
	libx11-6 \
	libx11-xcb1 \
	libxcb1 \
	libxcomposite1 \
	libxcursor1 \
	libxdamage1 \
	libxext6 \
	libxfixes3 \
	libxi6 \
	libxrandr2 \
	libxrender1 \
	libxss1 \
	libxtst6 \
	ca-certificates \
	fonts-liberation \
	libappindicator1 \
	libnss3 \
	lsb-release \
	xdg-utils\
    wget

# Install XVFB if there's a need to run browsers in headful mode
RUN apt-get install -y --no-install-recommends \
    xvfb \
    x11-apps \
    x11vnc


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
