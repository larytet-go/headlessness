FROM chromedp/headless-shell:latest


RUN apt-get update && apt-get install -y --no-install-recommends \
    dumb-init 

ENTRYPOINT ["dumb-init", "--"]
# CMD ["/path/to/your/program"]