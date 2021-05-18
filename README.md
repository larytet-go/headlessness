
Headlessness is an HTTP service wrapping the [ChromeDP](https://github.com/chromedp/chromedp/) plus performance related candies. Headlessness achieves sub-second simultaneous rendering of 10s of WEB pages, enforces deadlines, mitigates browser failures.

## Usage


### Container 
```
# go fmt .
docker build -t headlessness .
docker run --shm-size 2G --rm -p 5900:5900 -p 8081:8081 --init headlessness
curl --silent --data '{"urls":["https://www.google.com/", "https://www.google.com/search?q=test1", "https://www.google.com/search?q=test2"]}' "http://0.0.0.0:8081/fetch?deadline=3000&transaction_id=1"

curl --silent -X POST "http://0.0.0.0:8081/fetch?url=https%253A%252F%252Fmeyerweb.com%252Feric%252Ftools%252Fdencoder%252F&deadline=3000&transaction_id=1"


```

A single URL from the command line

``` 
docker run --shm-size 2G --rm -p 5900:5900 -p 8081:8081 --init --entrypoint=""  headlessness  /home/chrome/headlessness  -url="https://www.google.com/search?q=test1" | ./headlessness -parseReport=true -dumpFilename=google1
```

Try VNC 127.0.0.1:5900
```
remmina -c $PWD/local-chrome.remmina
```

End to end test 
```
clear && go fmt . && go fmt ./chrome/.&& GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -a -o . ./  && docker build -t headlessness . && docker run  --shm-size 2G  -p 5900:5900 -p 8081:8081 --init --entrypoint=""  headlessness  /home/chrome/headlessness  -url="https://www.google.com/search?q=test1" | ./headlessness -parseReport=true -dumpFilename=google3
```

### Report dump tool

```
go mod download
GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -a -o . ./
curl --silent --data '{"urls":["https://www.google.com/"]}' "http://0.0.0.0:8081/fetch" | ./headlessness -parseReport=true -dumpFilename=google

# Tip: use eog to display images in Ubuntu
eog google.*.png 
```


Stats
```
while [ 1 ];do echo -en "\\033[0;0H";curl "http://0.0.0.0:8081/stats?format=text";sleep 0.2;done
```


## TODO

* Adblock
* Persistent cache
* A coomand line interface for a single URL
* Screenshot of the whole browser
* Custom Chrome build allowing display and VNC


## Links

* https://github.com/chromedp/docker-headless-shell
* https://pkg.go.dev/github.com/chromedp/chromedp
* https://github.com/chromedp/chromedp/issues/128
* https://github.com/raff/godet
* https://github.com/adieuadieu/serverless-chrome  - Lambda ready Chrome?
* https://pkg.go.dev/github.com/chromedp/cdproto/cachestorage  - cache support