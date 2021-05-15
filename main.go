// Command screenshot is a chromedp example demonstrating how to take a
// screenshot of a specific element and of the entire browser viewport.
package main

import (
	"context"
	"encoding/base64"
	"log"

	. "github.com/chromedp/chromedp"
	"time"
)

type contextWithCancel struct {
	ctx context.Context
	cancel context.CancelFunc
}
type Browser struct {
	contextWithCancel execAllocator
	contextWithCancel browserContext
}


func  getChromeOpions() []ExecAllocatorOption {
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

func New() (browser *Browser) {	
	browser := &Browser{}
	// https://github.com/puppeteer/puppeteer/blob/main/docs/troubleshooting.md#setting-up-chrome-linux-sandbox
	opts := getChromeOpions()
	// create context
	browser.execAllocator.ctx, browser.execAllocator.cancel = NewExecAllocator(context.Background(), opts...)
	browser.browserContext.ctx, browser.browserContext.cancel = NewContext(
		allocCtx,
		WithErrorf(log.Printf), //WithDebugf(log.Printf),
	)

	// Load the browser the very first time
	if err, _ := browser.report ("https://www.google.coom"); err != nil {
		log.Fatal(err)
	}
	return browser
}

func (b *Browser) report(url string) (report string, err error) {
	var buf []byte
	if err := Run(ctx, fullScreenshot(`https://brank.as/`, 100, &buf)); err != nil {
		log.Fatal(err)
	}

	str := base64.StdEncoding.EncodeToString(buf)
}

func main() {
	browser = New()
	rpeort, err := browser.report()
	log.Printf(rpeort)
	for {
		time.Sleep(2 * time.Second)
	}
}

// fullScreenshot takes a screenshot of the entire browser viewport.
// Note: chromedp.FullScreenshot overrides the device's emulation settings. Reset
func fullScreenshot(urlstr string, quality int, res *[]byte) Tasks {
	return Tasks{
		Navigate(urlstr),
		FullScreenshot(res, quality),
	}
}
