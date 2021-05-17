package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"headlessness/chrome"
)

type HTTPHandler struct {
	browser *chrome.Browser
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

func (h *HTTPHandler) sendReport(w http.ResponseWriter, reports *chrome.Reports) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	count, err := w.Write(reports.ToJSON(true))
	if err != nil {
		err := fmt.Errorf("Failed to write report for transactionID=%s url=%v to the peer : %v, count=%d", reports.TransactionID, reports.URLs, err, count)
		log.Printf(err.Error())
	}
}

func getDeadline(r *http.Request) time.Duration {
	deadlines, ok := r.URL.Query()["deadline"]
	if !ok {
		deadlines = []string{"5000"}
	}
	deadline, err := strconv.Atoi(deadlines[0])
	if err != nil {
		log.Printf("Failed to parse deadline %s\n", deadlines[0])
		deadline = 5000
	}
	return time.Duration(deadline) * time.Millisecond
}

func getTransactionID(r *http.Request) string {
	transactionsID, ok := r.URL.Query()["transaction_id"]
	if !ok {
		transactionsID = []string{""}
	}
	transactionID := transactionsID[0]
	return transactionID
}

func getURLs(r *http.Request) (urls []string, err error) {
	urls = []string{}
	urlsEncoded, ok := r.URL.Query()["url"]
	if !ok {
		return
	}
	for _, urlEncoded := range urlsEncoded {
		var urlDecoded string
		urlDecoded, err = url.QueryUnescape(urlEncoded)
		if err != nil {
			err = fmt.Errorf("Failed to decode URL %v: %v", urlEncoded, err)
			return
		}
		urls = append(urls, urlDecoded)
	}

	return
}

func (h *HTTPHandler) report(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	maxURLs, activeTabs := h.browser.MaxTabs, int(h.browser.ActiveTabs)

	urls, err := getURLs(r)
	if err != nil {
		h._400(w, err)
		return
	}

	urlsCount := len(urls)
	if urlsCount+activeTabs > maxURLs {
		err := fmt.Errorf("Too many 'url' parameters in %v, max is %d, active tabs %d", r.URL.RawQuery, maxURLs, activeTabs)
		h._400(w, err)
		return
	}
	if urlsCount == 0 {
		err := fmt.Errorf("URL is missing in %v", r.URL.RawQuery)
		h._400(w, err)
		return
	}

	transactionID := getTransactionID(r)
	deadline := getDeadline(r)

	reports, err := h.browser.AsyncReports(transactionID, urls, deadline)
	if err != nil {
		err := fmt.Errorf("Failed to fetch transactionID %s, URLs %v: %v", transactionID, urls, err)
		h._500(w, err)
		return
	}
	reports.Elapsed = time.Since(startTime).Milliseconds()

	startTime = time.Now()
	h.sendReport(w, reports)
}

func (h *HTTPHandler) stats(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Ok"))
}

func (h *HTTPHandler) screenshot(w http.ResponseWriter, r *http.Request) {
}
