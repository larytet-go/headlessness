// Command screenshot is a chromedp example demonstrating how to take a
// screenshot of a specific element and of the entire browser viewport.
package chrome

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/url"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/cdp"
	// "github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
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
// This code is not in use
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
	MaxTabs        int
	browserContext contextWithCancel
	browserTab     contextWithCancel
	ActiveTabs     int64
	adBlock        AdBlockIfc
	skipCategory   map[string]struct{}
}

type Request struct {
	URL          string               `json:"url"`
	TSRequest    time.Time            `json:"ts_request"`
	TSResponse   time.Time            `json:"ts_respons"`
	Elapsed      int                  `json:"elapsed"`
	Status       int64                `json:"status"`
	ResponseData []byte               `json:"response_data"`
	FromCache    bool                 `json:"from_cache"`
	IsAd         bool                 `json:"is_ad"`
	SlowHost     bool                 `json:"slow_host"`
	ResourceType network.ResourceType `json:"resource_type"`
	Blocked      bool                 `json:"blocked"`
	MIME         string               `json:"mime"`
}

const (
	WebPageCategoryMedia = "Media"
)

type Report struct {
	URL             string         `json:"url"`
	TransactionID   string         `json:"transaction_id"`
	RequestID       string         `json:"request_id"`
	Requests        []Request      `json:"requests"`
	Redirects       []string       `json:"redirects"`
	SlowResponses   []string       `json:"slow_responses"`
	Ads             []string       `json:"ads"`
	Screenshot      string         `json:"screenshot"`
	Content         string         `json:"content"`
	Errors          string         `json:"errors"`
	Elapsed         int64          `json:"elapsed"`
	RequestsElapsed int64          `json:"requests_elapsed"`
	WebPageCategory string         `json:"web_page_category"`
	SkipCategory    bool           `json:"skip_category"`
	Dependencies    []string       `json:"dependencies"`
	Metrics         map[string]int `json:"metrics"`
}

func (r *Report) ToJSON(pretty bool) (s []byte) {
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

// see: https://intoli.com/blog/not-possible-to-block-chrome-headless/
const counterDetectionScript = `(function(w, n, wn) {
	// Pass the Webdriver Test.
	Object.defineProperty(n, 'webdriver', {
	  get: () => false,
	});
  
	// Pass the Plugins Length Test.
	// Overwrite the plugins property to use a custom getter.
	Object.defineProperty(n, 'plugins', {
	  // This just needs to have length > 0 for the current test,
	  // but we could mock the plugins too if necessary.
	  get: () => [1, 2, 3, 4, 5],
	});
  
	// Pass the Languages Test.
	// Overwrite the plugins property to use a custom getter.
	Object.defineProperty(n, 'languages', {
	  get: () => ['en-US', 'en'],
	});
  
	// Pass the Chrome Test.
	// We can mock this in as much depth as we need for the test.
	w.chrome = {
	  runtime: {},
	};
  
	// Pass the Permissions Test.
	const originalQuery = wn.permissions.query;
	return wn.permissions.query = (parameters) => (
	  parameters.name === 'notifications' ?
		Promise.resolve({ state: Notification.permission }) :
		originalQuery(parameters)
	);
  
  })(window, navigator, window.navigator);`

func getChromeOpions() []ExecAllocatorOption {
	return []ExecAllocatorOption{
		NoFirstRun,
		NoDefaultBrowserCheck,
		NoSandbox,
		WindowSize(1920, 1080),
		// Headless,
		// See counter headless detection https://github.com/chromedp/chromedp/issues/396
		UserAgent("Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:88.0) Gecko/20100101 Firefox/88.0"),
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
		Flag("ignore-certificate-errors", true),
	}
}

type BrowserParams struct {
	// WEB site categories for which I can skip screenshot
	SkipCategory []string
}

func New(params BrowserParams) (browser *Browser, err error) {
	maxTabs := runtime.NumCPU()
	browser = &Browser{
		MaxTabs:      maxTabs,
		skipCategory: map[string]struct{}{},
	}

	// https://github.com/puppeteer/puppeteer/blob/main/docs/troubleshooting.md#setting-up-chrome-linux-sandbox
	opts := getChromeOpions()
	browser.browserContext.ctx, browser.browserContext.cancel = NewExecAllocator(context.Background(), opts...)

	if params.SkipCategory != nil {
		for _, category := range params.SkipCategory {
			browser.skipCategory[category] = struct{}{}
		}
	}

	// create contexts
	// https://github.com/chromedp/chromedp/issues/821
	browser.browserTab.ctx, browser.browserTab.cancel = NewContext(browser.browserContext.ctx,
		WithErrorf(log.Printf), //WithErrorf, WithDebugf
	)
	// Run the browser in the first time
	// https://github.com/chromedp/chromedp/issues/824
	Run(browser.browserTab.ctx)

	browser.adBlock, err = NewAdBlockList([]string{"./ads-servers.txt", "./ads-servers.he.txt"})
	if err != nil {
		return
	}

	return
}

// See implementation https://github.com/chromedp/chromedp/blob/master/emulate.go#L129
// TODO Refactor according to https://github.com/chromedp/chromedp/issues/826
func fullScreenshot(ch chan []byte) EmulateAction {
	screenshot := []byte{}
	return ActionFunc(func(ctx context.Context) error {
		go func() {
			// get layout metrics
			_, _, contentSize, _, _, cssContentSize, err := page.GetLayoutMetrics().Do(ctx)
			if err != nil {
				return
			}
			// protocol v90 changed the return parameter name (contentSize -> cssContentSize)
			if cssContentSize != nil {
				contentSize = cssContentSize
			}
			width, height := int64(math.Ceil(contentSize.Width)), int64(math.Ceil(contentSize.Height))
			// force viewport emulation
			err = emulation.SetDeviceMetricsOverride(width, height, 1, false).
				WithScreenOrientation(&emulation.ScreenOrientation{
					Type:  emulation.OrientationTypePortraitPrimary,
					Angle: 0,
				}).
				Do(ctx)
			if err != nil {
				return
			}
			// capture screenshot
			screenshot, err = page.CaptureScreenshot().
				WithQuality(10).
				WithClip(&page.Viewport{
					X:      contentSize.X,
					Y:      contentSize.Y,
					Width:  contentSize.Width,
					Height: contentSize.Height,
					Scale:  1,
				}).Do(ctx)
			if err != nil {
				return
			}
			ch <- screenshot
		}()

		return nil
	})
}

// Return actions scrapping a WEB page, collecting HTTP requests
func scrapPage(urlstr string, screenshotCh chan []byte, content *string, errors *string) Tasks {
	return Tasks{
		ActionFunc(func(ctx context.Context) error {
			scriptID, err := page.AddScriptToEvaluateOnNewDocument(counterDetectionScript).Do(ctx)
			if err != nil {
				*errors += fmt.Sprintf("AddScriptToEvaluateOnNewDocument %v:", scriptID) + err.Error() + ". "
				return err
			}
			return nil
		}),
		network.Enable(),
		fetch.Enable().WithPatterns([]*fetch.RequestPattern{
			{
				ResourceType: network.ResourceTypeScript,
				RequestStage: fetch.RequestStageRequest,
			},
			{
				ResourceType: network.ResourceTypeImage,
				RequestStage: fetch.RequestStageRequest,
			},
			{
				ResourceType: network.ResourceTypeMedia,
				RequestStage: fetch.RequestStageRequest,
			},
			{
				ResourceType: network.ResourceTypeFont,
				RequestStage: fetch.RequestStageRequest,
			},
		}),
		Navigate(urlstr),
		fullScreenshot(screenshotCh),
		// https://github.com/chromedp/chromedp/blob/master/example_test.go
		// https://github.com/chromedp/examples/blob/master/subtree/main.go
		// https://github.com/chromedp/chromedp/issues/128
		// https://github.com/chromedp/chromedp/issues/370
		// https://pkg.go.dev/github.com/chromedp/chromedp#example-package--RetrieveHTML
		// https://github.com/chromedp/chromedp/issues/128
		// https://github.com/chromedp/chromedp/issues/762
		OuterHTML("html", content, ByQuery),
	}
}

type webPageMetrics struct {
	mutex        sync.Mutex
	objects      map[string]int
	dependencies map[string]struct{}
}

func newWebPageMetrics() webPageMetrics {
	return webPageMetrics{
		objects:      map[string]int{},
		dependencies: map[string]struct{}{},
	}
}

func (wpm webPageMetrics) incMetric(metric string) int {
	wpm.mutex.Lock()
	defer wpm.mutex.Unlock()
	if counter, ok := wpm.objects[metric]; ok {
		wpm.objects[metric] = counter + 1
	} else {
		wpm.objects[metric] = 1
	}
	return wpm.objects[metric]
}

func (wpm webPageMetrics) getMetric(metric network.ResourceType) int {
	if counter, ok := wpm.objects[string(metric)]; ok {
		return counter
	}
	return 0
}

// https://pkg.go.dev/github.com/chromedp/cdproto/network#ResourceType
// https://web.eecs.umich.edu/~harshavm/papers/imc11.pdf
func (wpm webPageMetrics) isNewsSite() bool {
	return wpm.getMetric(network.ResourceTypeImage) > 40 &&
		wpm.getMetric(network.ResourceTypeFont) > 3 &&
		wpm.getMetric(network.ResourceTypeScript) > 20 &&
		wpm.getMetric("ad") > 4 &&
		len(wpm.dependencies) > 10
}

type eventListener struct {
	mutex             sync.Mutex
	urls              map[string]struct{}
	redirects         []string
	requests          map[network.RequestID]*Request
	ctx               context.Context
	adBlock           AdBlockIfc
	webPageCategoryCh chan string

	webPageMetrics  webPageMetrics
	webPageCategory string
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

func (el *eventListener) isAd(requestURL string) (bool, string) {
	u, err := url.Parse(requestURL)
	return err == nil && el.adBlock.IsAd(u.Host), u.Host
}

// I need fetch domain enable for all URLs, and EventRequestPaused
// Any additional listener slowes down the requests
func (el *eventListener) requestPaused(ev *fetch.EventRequestPaused) {
	now := time.Now()
	requestURL := ev.Request.URL
	isAd, hostname := el.isAd(requestURL)
	el.webPageMetrics.dependencies[hostname] = struct{}{}
	requestID := (ev.RequestID)
	resourceType := ev.ResourceType
	blocked := isAd ||
		resourceType == network.ResourceTypeXHR ||
		resourceType == network.ResourceTypeMedia

	el.webPageMetrics.incMetric(string(resourceType))
	if isAd {
		el.webPageMetrics.incMetric("ad")
	}

	// https://stackoverflow.com/questions/5216831/can-we-measure-complexity-of-web-site/13674590#13674590
	// https://web.eecs.umich.edu/~harshavm/papers/imc11.pdf
	if el.webPageCategory == "" && el.webPageMetrics.isNewsSite() {
		el.webPageCategory = WebPageCategoryMedia
		el.webPageCategoryCh <- el.webPageCategory
	}

	if blocked {
		go el.failRequest(requestURL, requestID)
	} else {
		go el.continueRequest(requestURL, requestID)
	}

	el.mutex.Lock()
	defer el.mutex.Unlock()

	if request, ok := el.requests[network.RequestID(requestID)]; ok {
		request.IsAd = isAd
		request.ResourceType = resourceType
		request.Blocked = blocked
	} else {
		el.requests[network.RequestID(requestID)] = &Request{
			URL:          requestURL,
			TSRequest:    now,
			IsAd:         isAd,
			ResourceType: resourceType,
			Blocked:      blocked,
		}
	}
}

func (el *eventListener) failRequest(requestURL string, requestID fetch.RequestID) {
	chromedpContext := FromContext(el.ctx)
	execCtx := cdp.WithExecutor(el.ctx, chromedpContext.Target)
	err := fetch.FailRequest(requestID, network.ErrorReasonFailed).Do(execCtx)
	if err != nil {
		log.Printf("Failed to abort the request for url %v: %v", requestURL, err)
	}
}

func (el *eventListener) continueRequest(requestURL string, requestID fetch.RequestID) {
	chromedpContext := FromContext(el.ctx)
	execCtx := cdp.WithExecutor(el.ctx, chromedpContext.Target)
	err := fetch.ContinueRequest(requestID).Do(execCtx)
	if err != nil {
		log.Printf("Failed to continue the request for url %v: %v", requestURL, err)
	}
}

func (el *eventListener) requestWillBeSent(ev *network.EventRequestWillBeSent) {
	now := time.Now()
	requestID := ev.RequestID
	requestURL := ev.Request.URL

	redirectResponse := ev.RedirectResponse

	el.mutex.Lock()
	defer el.mutex.Unlock()

	isAd, hostname := el.isAd(requestURL)
	el.webPageMetrics.dependencies[hostname] = struct{}{}
	if _, ok := el.requests[requestID]; !ok {
		el.requests[requestID] = &Request{
			URL:          requestURL,
			TSRequest:    now,
			IsAd:         isAd,
			ResourceType: ev.Type,
		}
	}
	if redirectResponse != nil {
		el.redirects = append(el.redirects, redirectResponse.URL)
	}
}

func (el *eventListener) responseReceived(ev *network.EventResponseReceived) {
	now := time.Now()
	requestID := ev.RequestID
	url := ev.Response.URL
	mime := ev.Response.MimeType
	el.webPageMetrics.incMetric(mime)

	el.mutex.Lock()
	defer el.mutex.Unlock()

	if _, ok := el.requests[requestID]; !ok {
		log.Printf("Request %s [%s] is missing in the map", url, requestID)
		return
	}
	request := el.requests[requestID]
	request.Status = ev.Response.Status
	request.TSResponse = now
	request.Elapsed = int(time.Since(request.TSRequest) / time.Millisecond)
	request.SlowHost = request.Elapsed > 1000
	request.MIME = mime
}

func (el *eventListener) requestServedFromCache(ev *network.EventRequestServedFromCache) {
	now := time.Now()
	requestID := ev.RequestID

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

func (b *Browser) Report(url string, deadline time.Duration) (report *Report, err error) {
	atomic.AddInt64(&b.ActiveTabs, 1)
	defer atomic.AddInt64(&b.ActiveTabs, -1)
	report = &Report{URL: url,
		Requests: []Request{},
	}

	tabContext := contextWithCancel{}
	// Allocate a free tab from the pool of the browser tabs
	tabContext.ctx, tabContext.cancel = NewContext(b.browserTab.ctx)
	defer tabContext.cancel()

	webPageCategoryCh := make(chan string, 1)
	eventListener := &eventListener{
		requests:          map[network.RequestID]*Request{},
		redirects:         []string{},
		urls:              map[string]struct{}{},
		ctx:               tabContext.ctx,
		adBlock:           b.adBlock,
		webPageCategoryCh: webPageCategoryCh,
		webPageMetrics:    newWebPageMetrics(),
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
		case *fetch.EventRequestPaused:
			eventListener.requestPaused(ev.(*fetch.EventRequestPaused))
		}
	})

	screenshotCh := make(chan []byte, 1)
	var content string
	var errors string
	if err = Run(tabContext.ctx, scrapPage(url, screenshotCh, &content, &errors)); err != nil {
		return
	}

	screenshot := []byte{}
	var webPageCategory string
	select {
	case webPageCategory = <-webPageCategoryCh:
		if _, skipCategory := b.skipCategory[webPageCategory]; skipCategory {
			report.SkipCategory = true
			break
		}
	case screenshot = <-screenshotCh:
		break
	case <-time.After(deadline):
		break
	}

	report.Screenshot = base64.StdEncoding.EncodeToString(screenshot)
	report.Content = base64.StdEncoding.EncodeToString([]byte(content))
	report.Errors = errors
	report.Requests = eventListener.dumpCollectedRequests()
	report.Redirects = eventListener.redirects
	report.WebPageCategory = webPageCategory
	report.Dependencies = []string{}
	for hostname := range eventListener.webPageMetrics.dependencies {
		if hostname == "" {
			continue
		}
		report.Dependencies = append(report.Dependencies, hostname)
	}
	report.Metrics = map[string]int{}
	for metric, value := range eventListener.webPageMetrics.objects {
		if metric == "" {
			continue
		}
		report.Metrics[string(metric)] = value
	}

	return
}

func (b *Browser) close() {
	b.browserTab.cancel()
	b.browserContext.cancel()
}

func (b *Browser) asyncReport(transactionID string, url string, result chan *Report, deadline time.Duration) {

	startTime := time.Now()
	report, err := b.Report(url, deadline)
	if err != nil {
		err := fmt.Errorf("Failed to fetch URL %v: %v", url, err)
		log.Printf(err.Error())
		report.Errors += err.Error() + "."
	}

	report.TransactionID = transactionID
	report.URL = url
	report.Elapsed = time.Since(startTime).Milliseconds()

	maxTime := time.Now().Add(-deadline)
	minTime := time.Now()
	for _, request := range report.Requests {
		if request.TSRequest.Before(minTime) {
			minTime = request.TSRequest
		}
		if request.TSResponse.After(maxTime) {
			maxTime = request.TSRequest
		}
	}
	if len(report.Requests) > 0 {
		report.RequestsElapsed = int64(maxTime.Sub(minTime) / time.Millisecond)
	}
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
