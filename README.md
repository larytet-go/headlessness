
## Usage

```
# go fmt .
docker build -t headlessness .
docker run --shm-size 8G --rm -p 5900:5900 -p 8081:8081 --init headlessness
curl --silent -X POST "http://0.0.0.0:8081/fetch?url=https%3A%2F%2Fwww.w3schools.com%2F&transaction_id=1"
```

Try VNC 127.0.0.1:5900
```
remmina -c $PWD/local-chrome.remmina
```


Stats
```
while [ 1 ];do echo -en "\\033[0;0H";curl "http://0.0.0.0:8081/stats?format=text";sleep 0.2;done
```


## Links

* https://github.com/chromedp/docker-headless-shell
* https://pkg.go.dev/github.com/chromedp/chromedp
* https://github.com/chromedp/chromedp/issues/128
* https://github.com/raff/godet