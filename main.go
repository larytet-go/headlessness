// Command screenshot is a chromedp example demonstrating how to take a
// screenshot of a specific element and of the entire browser viewport.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"

	. "github.com/chromedp/chromedp"
)

type contextWithCancel struct {
	ctx    context.Context
	cancel context.CancelFunc
}

type Browser struct {
	execAllocator  contextWithCancel
	browserContext contextWithCancel
}

type Request struct {
	URL string
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
}

func (r *Report) toJSON() []byte {
	s, _ := json.Marshal(r)
	return s
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
		WithErrorf(log.Printf), //WithDebugf(log.Printf),
	)

	// Load the browser the very first time
	_, err = browser.report(`https://www.google.com`)
	return
}

// fullScreenshot takes a screenshot of the entire browser viewport.
// Note: chromedp.FullScreenshot overrides the device's emulation settings. Reset
func fullScreenshot(urlstr string, quality int, res *[]byte) Tasks {
	return Tasks{
		Navigate(urlstr),
		FullScreenshot(res, quality),
	}
}

func (b *Browser) report(url string) (report *Report, err error) {
	var buf []byte
	if err = Run(b.browserContext.ctx, fullScreenshot(url, 100, &buf)); err != nil {
		return
	}
	report = &Report{}
	report.URL = url
	report.Screenshot = base64.StdEncoding.EncodeToString(buf)
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
	report, err := browser.report(`https://www.google.com`)
	if err != nil {
		log.Printf(err.Error())
		return
	}

	fmt.Printf("%s\n", report.toJSON())
	browser.close()
	for {
		time.Sleep(time.Second)
	}
}
