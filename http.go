package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/larytet-go/headlessness/chrome"
)

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

func (h *HTTPHandler) sendReport(w http.ResponseWriter, reports *Reports) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	count, err := w.Write(reports.toJSON(true))
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

func (h *HTTPHandler) report(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	urlsEncoded, ok := r.URL.Query()["url"]
	if !ok {
		err := fmt.Errorf("URL is missing in %v", r.URL.RawQuery)
		h._400(w, err)
		return
	}
	maxURLs := h.browser.poolOfBrowserTabs.size
	urlsCount := len(urlsEncoded)
	if urlsCount > maxURLs {
		err := fmt.Errorf("Too many 'url' parameters in %v, max is %d", r.URL.RawQuery, maxURLs)
		h._400(w, err)
		return
	}

	urlsDecoded := make([]string, urlsCount)
	for i, urlEncoded := range urlsEncoded {
		urlDecoded, err := url.QueryUnescape(urlEncoded)
		if err != nil {
			err := fmt.Errorf("Failed to decode URL %v: %v", urlEncoded, err)
			h._400(w, err)
			return
		}
		urlsDecoded[i] = urlDecoded
	}

	transactionID := getTransactionID(r)
	deadline := getDeadline(r)

	reports, err := h.browser.AsyncReports(transactionID, urlsDecoded, deadline)
	if err != nil {
		err := fmt.Errorf("Failed to fetch transactionID %s, URLs %v: %v", transactionID, urlsDecoded, err)
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
