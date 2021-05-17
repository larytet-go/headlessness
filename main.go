package main

import (
	"log"
	"net/http"
	"time"

	"headlessness/chrome"
)

func main() {
	browser, err := chrome.New()
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
	mux.HandleFunc("/screenshot", httpHandler.screenshot)

	httpServer := http.Server{
		Addr:           ":8081",
		Handler:        mux,
		ReadTimeout:    1 * time.Second,
		WriteTimeout:   100 * time.Second,
		MaxHeaderBytes: 1 << 28,
	}
	log.Fatal(httpServer.ListenAndServe())
}
