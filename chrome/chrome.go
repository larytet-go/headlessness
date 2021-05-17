// Command screenshot is a chromedp example demonstrating how to take a
// screenshot of a specific element and of the entire browser viewport.
package chrome

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

type PoolOfBrowserTabs struct {
	size     int
	top      int
	contexts []contextWithCancel
	mutex    sync.Mutex
}

// Returns a pool (stack) of browser tabs
func NewPoolOfBrowserTabs(newExecAllocator context.Context, size int) *PoolOfBrowserTabs {
	contexts := make([]contextWithCancel, size)

	for i := 0; i < size; i++ {
		tabContext := contextWithCancel{}
		tabContext.ctx, tabContext.cancel = NewContext(
			newExecAllocator,
			WithErrorf(log.Printf), //WithErrorf, WithDebugf
		)
		contexts[i] = tabContext
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
	MaxTabs int
	browserContext    contextWithCancel
	browserTab contextWithCancel
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

type Reports struct {
	Count         int       `json:"total"`
	URLReports    []*Report `json:"reports"`
	Elapsed       int64     `json:"elapsed"`
	TransactionID string    `json:"transaction_id"`
	URLs          []string  `json:"urls"`
	Errors        string    `json:"errors"`
}

func (r *Reports) ToJSON(pretty bool) (s []byte) {
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
		// Flag("remote-debugging-port", "9222"), https://github.com/chromedp/chromedp/issues/821
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
	maxTabs := 12
	browser = &Browser{MaxTabs: maxTabs}

	// https://github.com/puppeteer/puppeteer/blob/main/docs/troubleshooting.md#setting-up-chrome-linux-sandbox
	opts := getChromeOpions()
	browser.browserContext.ctx, browser.browserContext.cancel = NewExecAllocator(context.Background(), opts...)

	// create contexts
	browser.browserTab.ctx, browser.browserTab.cancel = NewContext(browser.browserContext.ctx, 
		WithErrorf(log.Printf), //WithErrorf, WithDebugf
	)

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

func (b *Browser) report(url string, deadline time.Duration) (report *Report, err error) {
	report = &Report{URL: url,
		Requests: []Request{},
	}

	tabContext := contextWithCancel{}
	// Allocate a free tab from the pool of the browser tabs
	tabContext.ctx, tabContext.cancel = NewContext(b.browserTab.ctx, 
		WithErrorf(log.Printf), //WithErrorf, WithDebugf)
	)
	defer tabContext.cancel()

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

	return
}

func (b *Browser) close() {
	b.poolOfBrowserTabs.close()
	b.browserContext.cancel()
}

func (b *Browser) asyncReport(transactionID string, url string, result chan *Report, deadline time.Duration) {
	startTime := time.Now()
	report, err := b.report(url, deadline)
	if err != nil {
		err := fmt.Errorf("Failed to fetch URL %v: %v", url, err)
		log.Printf(err.Error())
		report.Errors += err.Error() + "."
	}

	report.TransactionID = transactionID
	report.URL = url
	report.Elapsed = time.Since(startTime).Milliseconds()
	result <- report
}

func (b *Browser) AsyncReports(transactionID string, urls []string, deadline time.Duration) (reports *Reports, err error) {
	urlsCount := len(urls)
	reportsCh := make(chan *Report, urlsCount)

	for _, url := range urls {
		go b.asyncReport(transactionID, url, reportsCh, deadline)
	}

	reports = &Reports{
		Count:         urlsCount,
		URLReports:    []*Report{},
		TransactionID: transactionID,
		URLs:          urls,
	}

	processedURLs := map[string]struct{}{}
	for i := 0; i < urlsCount; i++ {
		select {
		case report := <-reportsCh:
			reports.URLReports = append(reports.URLReports, report)
			processedURLs[report.URL] = struct{}{}
			log.Printf("Report is completed for transactionID %s url %s, %d ms", transactionID, report.URL, report.Elapsed)
			continue
		case <-time.After(deadline):
			break
		}
	}

	for _, url := range reports.URLs {
		if _, ok := processedURLs[url]; ok {
			continue
		}
		reports.Errors += reports.Errors + fmt.Sprintf("URL %s hit deadline %ds. ", url, deadline/time.Millisecond)
	}

	// Drop the remaining URLs, close the channel
	go func() {
		for i := 0; i < urlsCount-len(reports.URLReports); i++ {
			report := <-reportsCh
			log.Printf("Report hit deadline %ds for transactionID %s url %s, processing time %d ms", deadline/time.Millisecond, transactionID, report.URL, report.Elapsed)
		}
		close(reportsCh)
	}()

	return
}
