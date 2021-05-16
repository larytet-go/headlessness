// Command screenshot is a chromedp example demonstrating how to take a
// screenshot of a specific element and of the entire browser viewport.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
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

type PoolOfBrowserTabs struct {
	size     int
	top      int
	contexts []contextWithCancel
	mutex    sync.Mutex
}

func NewPoolOfBrowserTabs(size int) *PoolOfBrowserTabs {
	contexts := make([]contextWithCancel, size)

	// https://github.com/puppeteer/puppeteer/blob/main/docs/troubleshooting.md#setting-up-chrome-linux-sandbox
	opts := getChromeOpions()

	for i := 0; i < size; i++ {
		browserCtx := contextWithCancel{}
		browserCtx.ctx, browserCtx.cancel = NewExecAllocator(context.Background(), opts...)
		contexts[i] = browserCtx
	}
	return &PoolOfBrowserTabs{
		size:     size,
		top:      size,
		contexts: contexts,
	}
}

func (p *PoolOfBrowserTabs) close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for i := 0; i < p.top; i++ {
		ctx := p.contexts[i]
		ctx.cancel()
	}
	p.size = 0
	p.top = 0
}

func (p *PoolOfBrowserTabs) pop() (ctx contextWithCancel, err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.top == 0 {
		return ctx, fmt.Errorf("Empty")
	}
	p.top -= 1
	return p.contexts[p.top], nil
}

func (p *PoolOfBrowserTabs) push(ctx contextWithCancel) (err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.top == p.size {
		return fmt.Errorf("Full")
	}
	p.contexts[p.top] = ctx
	p.top += 1
	return
}

type Browser struct {
	poolOfBrowserTabs *PoolOfBrowserTabs
}

type Request struct {
	URL          string    `json:"url"`
	TSRequest    time.Time `json:"ts_request"`
	TSResponse   time.Time `json:"ts_respons"`
	Status       int64     `json:"status"`
	ResponseData []byte    `json:"response_data"`
	FromCache    bool      `json:"from_cache"`
}

type Report struct {
	URL           string    `json:"url"`
	TransactionID string    `json:"transaction_id"`
	RequestID     string    `json:"request_id"`
	Requests      []Request `json:"requests"`
	Redirects     []string  `json:"redirects"`
	SlowResponses []string  `json:"slow_responses"`
	Ads           []string  `json:"ads"`
	Screenshot    string    `json:"screenshot"`
	Content       string    `json:"content"`
	Errors        string    `json:"errors"`
	Elapsed       int64     `json:"elapsed"`
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
	// create contexts
	browser.poolOfBrowserTabs = NewPoolOfBrowserTabs(12)

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

type eventListener struct {
	urls      map[string]struct{}
	redirects []string
	requests  map[network.RequestID]*Request
	mutex     sync.Mutex
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

	redirectResponse := r.RedirectResponse
	if request, ok := el.requests[requestID]; ok && redirectResponse == nil {
		log.Printf("Request %s [%s]  already is in the map for url %s: request=%v, event=%v", url, requestID, documentURL, request, r)
	}
	if redirectResponse != nil {
		el.redirects = append(el.redirects, redirectResponse.URL)
	}
	// log.Printf("Add request %s [%s]", url, requestID)
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
		log.Printf("Request %s [%s] is missing in the map", url, requestID)
		return
	}
	request := el.requests[requestID]
	request.Status = r.Response.Status
	request.TSResponse = now
}

func (el *eventListener) requestServedFromCache(r *network.EventRequestServedFromCache) {
	now := time.Now()
	requestID := r.RequestID

	el.mutex.Lock()
	defer el.mutex.Unlock()

	if _, ok := el.requests[requestID]; !ok {
		log.Printf("Request [%s] is missing in the map (served from cache)", requestID)
		return
	}
	request := el.requests[requestID]
	request.FromCache = true
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
	browserContext, err := b.poolOfBrowserTabs.pop()
	if err != nil {
		return report, fmt.Errorf("Too many tabs already")
	}
	defer b.poolOfBrowserTabs.push(browserContext)

	tabContext := contextWithCancel{}
	tabContext.ctx, tabContext.cancel = NewContext(
		browserContext.ctx,
		WithErrorf(log.Printf), //WithErrorf, WithDebugf
	)
	defer tabContext.cancel()

	log.Printf("Fetching the url %s", url)

	eventListener := &eventListener{
		requests:  map[network.RequestID]*Request{},
		redirects: []string{},
		urls:      map[string]struct{}{},
	}
	defer eventListener.removeDocumentURL(url)
	eventListener.addDocumentURL(url)

	// https://github.com/chromedp/chromedp/issues/679
	// https://github.com/chromedp/chromedp/issues/559
	// https://github.com/chromedp/chromedp/issues/180
	// https://pkg.go.dev/github.com/chromedp/chromedp#WaitNewTarget
	// https://github.com/chromedp/chromedp/issues/700 <-- abort request
	ListenTarget(tabContext.ctx, func(ev interface{}) {
		switch ev.(type) {
		case *network.EventRequestServedFromCache:
			eventListener.requestServedFromCache(ev.(*network.EventRequestServedFromCache))
		case *network.EventRequestWillBeSent:
			eventListener.requestWillBeSent(ev.(*network.EventRequestWillBeSent))
		case *network.EventResponseReceived:
			eventListener.responseReceived(ev.(*network.EventResponseReceived))
		}
	})

	var screenshot []byte
	var content string
	var errors string
	if err = Run(tabContext.ctx, scrapPage(url, &screenshot, &content, &errors)); err != nil {
		return
	}
	report.Screenshot = base64.StdEncoding.EncodeToString(screenshot)
	report.Content = base64.StdEncoding.EncodeToString([]byte(content))
	report.Errors = errors
	report.Requests = eventListener.dumpCollectedRequests()
	report.Redirects = eventListener.redirects

	log.Printf("Fetching the url %s completed", url)

	return
}

func (b *Browser) close() {
	b.poolOfBrowserTabs.close()
}

type HTTPHandler struct {
	browser *Browser
}

func (h *HTTPHandler) _400(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/plain")
	log.Printf(err.Error())
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
}

func (h *HTTPHandler) _500(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/plain")
	log.Printf(err.Error())
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
}

func (h *HTTPHandler) sendReport(w http.ResponseWriter, report *Report) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	count, err := w.Write(report.toJSON(true))
	if err != nil {
		err := fmt.Errorf("Failed to write report for %v to the peer : %v, count=%d", report.URL, err, count)
		log.Printf(err.Error())
	}
}

func (h *HTTPHandler) report(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	urlEncoded, ok := r.URL.Query()["url"]
	if !ok {
		err := fmt.Errorf("URL is missing in %v", r.URL.RawQuery)
		h._400(w, err)
		return
	}
	if len(urlEncoded) > 1 {
		err := fmt.Errorf("Too many 'url' parameters in %v", r.URL.RawQuery)
		h._400(w, err)
		return
	}

	urlDecoded, err := url.QueryUnescape(urlEncoded[0])
	if err != nil {
		err := fmt.Errorf("Failed to decode URL %v: %v", urlEncoded, err)
		h._400(w, err)
		return
	}

	transactionID, ok := r.URL.Query()["transactionID"]
	if !ok {
		transactionID = []string{""}
	}

	report, err := h.browser.report(urlDecoded)
	if err != nil {
		err := fmt.Errorf("Failed to fetch URL %v: %v", urlDecoded, err)
		h._500(w, err)
		return
	}

	report.TransactionID = transactionID[0]
	report.Elapsed = time.Since(startTime).Milliseconds()
	report.URL = urlDecoded
	log.Printf("Report is completed for %s, %d ms ", urlDecoded, report.Elapsed)

	h.sendReport(w, report)
	log.Printf("Report is sent for %s, %d ms ", urlDecoded, report.Elapsed)
}

func (h *HTTPHandler) stats(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Ok"))
}

func main() {
	browser, err := New()
	if err != nil {
		log.Printf(err.Error())
		return
	}

	httpHandler := &HTTPHandler{
		browser: browser,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/fetch", httpHandler.report)
	mux.HandleFunc("/stats", httpHandler.stats)

	httpServer := http.Server{
		Addr:           ":8081",
		Handler:        mux,
		ReadTimeout:    1 * time.Second,
		WriteTimeout:   100 * time.Second,
		MaxHeaderBytes: 1 << 28,
	}
	log.Fatal(httpServer.ListenAndServe())
}
