
## Usage

```
# go fmt .
docker build -t headlessness .
docker run --shm-size 2G --rm -p 5900:5900 -p 8081:8081 --init headlessness
```

Try VNC 127.0.0.1:5900
```
remmina -c $PWD/local-chrome.remmina
```


## Links

* https://github.com/chromedp/docker-headless-shell
* https://pkg.go.dev/github.com/chromedp/chromedp
* https://github.com/chromedp/chromedp/issues/128