// Command screenshot is a chromedp example demonstrating how to take a
// screenshot of a specific element and of the entire browser viewport.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	//"github.com/chromedp/cdproto/page"
	. "github.com/chromedp/chromedp"
)

type contextWithCancel struct {
	ctx    context.Context
	cancel context.CancelFunc
}

type eventListener struct {
	urls     map[string]struct{}
	requests map[network.RequestID]*Request
	mutex    sync.Mutex
}

type Browser struct {
	execAllocator  contextWithCancel
	browserContext contextWithCancel
	eventListener  *eventListener
}

type Request struct {
	URL          string    `json:"url"`
	TSRequest    time.Time `json:"ts_request"`
	TSResponse   time.Time `json:"ts_respons"`
	Status       int64     `json:"status"`
	ResponseData []byte    `json:"response_data"`
}

type Report struct {
	URL           string    `json:"url"`
	RequestID     string    `json:"request_id"`
	Requests      []Request `json:"requests"`
	Redirects     []string  `json:"redirects"`
	SlowResponses []string  `json:"slow_responses"`
	Ads           []string  `json:"ads"`
	Screenshot    string    `json:"screenshot"`
	Content       string    `json:"content"`
	Errors        string    `json:"errors"`
}

func (r *Report) toJSON(pretty bool) (s []byte) {
	if pretty {
		s, _ = json.MarshalIndent(r, "", "\t")
	} else {
		s, _ = json.Marshal(r)
	}
	return
}

func getChromeOpions() []ExecAllocatorOption {
	return []ExecAllocatorOption{
		NoFirstRun,
		NoDefaultBrowserCheck,
		NoSandbox,
		// Headless,
		Flag("remote-debugging-port", "9222"),
		Flag("disable-background-networking", true),
		Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		Flag("disable-background-timer-throttling", true),
		Flag("disable-backgrounding-occluded-windows", true),
		Flag("disable-breakpad", true),
		Flag("disable-client-side-phishing-detection", true),
		Flag("disable-default-apps", true),
		Flag("disable-dev-shm-usage", true),
		Flag("disable-extensions", true),
		Flag("disable-features", "site-per-process,Translate,BlinkGenPropertyTrees"),
		Flag("disable-hang-monitor", true),
		Flag("disable-ipc-flooding-protection", true),
		Flag("disable-popup-blocking", true),
		Flag("disable-prompt-on-repost", true),
		Flag("disable-renderer-backgrounding", true),
		Flag("disable-sync", true),
		Flag("force-color-profile", "srgb"),
		Flag("metrics-recording-only", true),
		Flag("safebrowsing-disable-auto-update", true),
		Flag("enable-automation", true),
		Flag("password-store", "basic"),
		Flag("use-mock-keychain", true),
	}
}

func New() (browser *Browser, err error) {
	browser = &Browser{}
	// https://github.com/puppeteer/puppeteer/blob/main/docs/troubleshooting.md#setting-up-chrome-linux-sandbox
	opts := getChromeOpions()
	// create context
	browser.execAllocator.ctx, browser.execAllocator.cancel = NewExecAllocator(context.Background(), opts...)
	browser.browserContext.ctx, browser.browserContext.cancel = NewContext(
		browser.execAllocator.ctx,
		WithLogf(log.Printf), //WithErrorf, WithDebugf
	)
	browser.eventListener = &eventListener{
		requests: map[network.RequestID]*Request{},
		urls:     map[string]struct{}{},
	}

	// https://github.com/chromedp/chromedp/issues/679
	// https://github.com/chromedp/chromedp/issues/559
	// https://github.com/chromedp/chromedp/issues/180
	// https://pkg.go.dev/github.com/chromedp/chromedp#WaitNewTarget
	// https://github.com/chromedp/chromedp/issues/700 <-- abort request
	ListenTarget(browser.execAllocator.ctx, func(ev interface{}) {
		switch ev.(type) {
		case *network.EventRequestWillBeSent:
			browser.eventListener.requestWillBeSent(ev.(*network.EventRequestWillBeSent))
		case *network.EventResponseReceived:
			browser.eventListener.responseReceived(ev.(*network.EventResponseReceived))
		}
	})

	// Load the browser the very first time
	_, err = browser.report(`https://www.google.com`)

	return
}

// Return actions scrapping a WEB page, collecting HTTP requests
func scrapPage(urlstr string, screenshot *[]byte, content *string, errors *string) Tasks {
	quality := 50

	return Tasks{
		network.Enable(),
		Navigate(urlstr),
		FullScreenshot(screenshot, quality),

		// https://github.com/chromedp/chromedp/blob/master/example_test.go
		// https://github.com/chromedp/examples/blob/master/subtree/main.go
		// https://github.com/chromedp/chromedp/issues/128
		// https://github.com/chromedp/chromedp/issues/370
		// https://pkg.go.dev/github.com/chromedp/chromedp#example-package--RetrieveHTML
		// https://github.com/chromedp/chromedp/issues/128
		ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}
			*content, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			if err != nil {
				*errors += "Content:" + err.Error() + ". "
			}
			return nil
		}),
	}
}

func (el *eventListener) addDocumentURL(url string) {
	el.mutex.Lock()
	defer el.mutex.Unlock()

	el.urls[url] = struct{}{}
}

func (el *eventListener) removeDocumentURL(url string) {
	el.mutex.Lock()
	defer el.mutex.Unlock()

	delete(el.urls, url)
}

func (el *eventListener) requestWillBeSent(r *network.EventRequestWillBeSent) {
	now := time.Now()
	documentURL := r.DocumentURL
	requestID := r.RequestID
	url := r.Request.URL

	el.mutex.Lock()
	defer el.mutex.Unlock()

	if _, ok := el.urls[documentURL]; !ok {
		return
	}

	if _, ok := el.requests[requestID]; ok {
		log.Printf("Request %s already in the map for url %s", url, documentURL)
	}
	el.requests[requestID] = &Request{
		URL:       r.Request.URL,
		TSRequest: now,
	}
}

func (el *eventListener) responseReceived(r *network.EventResponseReceived) {
	now := time.Now()
	requestID := r.RequestID
	url := r.Response.URL

	el.mutex.Lock()
	defer el.mutex.Unlock()

	if _, ok := el.requests[requestID]; !ok {
		log.Printf("Request %s is missing in the map", url)
		return
	}
	request := el.requests[requestID]
	request.Status = r.Response.Status
	request.TSResponse = now
}

func (el *eventListener) dumpCollectedRequests() (requests []Request) {
	requests = []Request{}
	el.mutex.Lock()
	defer el.mutex.Unlock()
	for _, r := range el.requests {
		requests = append(requests, *r)
	}
	return
}

func (b *Browser) report(url string) (report *Report, err error) {
	report = &Report{URL: url,
		Requests: []Request{},
	}

	b.eventListener.addDocumentURL(url)
	defer b.eventListener.removeDocumentURL(url)

	var screenshot []byte
	var content string
	var errors string
	if err = Run(b.browserContext.ctx, scrapPage(url, &screenshot, &content, &errors)); err != nil {
		return
	}
	report.Screenshot = base64.StdEncoding.EncodeToString(screenshot)
	report.Content = base64.StdEncoding.EncodeToString([]byte(content))
	report.Errors = errors
	report.Requests = b.eventListener.dumpCollectedRequests()

	return
}

func (b *Browser) close() {
	b.browserContext.cancel()
	b.execAllocator.cancel()
}

func main() {
	browser, err := New()
	if err != nil {
		log.Printf(err.Error())
		return
	}
	report, err := browser.report(`https://www.w3.org/Protocols/HTTP/Performance/microscape/`)
	if err != nil {
		log.Printf(err.Error())
		return
	}

	fmt.Printf("%s\n", report.toJSON(true))
	browser.close()
	for {
		time.Sleep(time.Second)
	}
}
