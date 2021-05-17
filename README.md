
Headlessness is an HTTP service wrapping the [ChromeDP](https://github.com/chromedp/chromedp/) plus performance related candies. Headlessness achieves sub-second simultaneous rendering of 10s of WEB pages, enforces deadlines, mitigates browser failures.

## Usage

```
# go fmt .
docker build -t headlessness .
docker run --shm-size 2G --rm -p 5900:5900 -p 8081:8081 --init headlessness
curl --silent -X POST "http://0.0.0.0:8081/fetch?url=https%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3D9B0eXmbrBIo&deadline=3000&transaction_id=1&"
```

Try VNC 127.0.0.1:5900
```
remmina -c $PWD/local-chrome.remmina
```


Stats
```
while [ 1 ];do echo -en "\\033[0;0H";curl "http://0.0.0.0:8081/stats?format=text";sleep 0.2;done
```


## TODO

* Adblock
* A command line unitility dumping the .png image and page content to files
* HTTP POST with JSON containing URLs
* Persistent cache
* Screenshot of the whole browser
* Custom Chrome build allowing display and VNC


## Links

* https://github.com/chromedp/docker-headless-shell
* https://pkg.go.dev/github.com/chromedp/chromedp
* https://github.com/chromedp/chromedp/issues/128
* https://github.com/raff/godet
* https://github.com/adieuadieu/serverless-chrome  - Lambda ready Chrome?
* https://pkg.go.dev/github.com/chromedp/cdproto/cachestorage  - cache support