package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"headlessness/chrome"
)

// Based on https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go/22892986#22892986
var filenameLetters = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randomFilename(l int) string {
	lenFilenameLetters := len(filenameLetters)
	runes := make([]rune, l)
	for i := range runes {
		runes[i] = filenameLetters[rand.Intn(lenFilenameLetters)]
	}
	return string(runes)
}

func dumpFile(base64String, dumpFilename, extension string) {
	data, err := base64.StdEncoding.DecodeString(base64String)
	if err != nil {
		log.Fatalf("Failed to decode %v %v", extension, err)
		return
	}

	filename := dumpFilename + "." + randomFilename(4) + "." + extension
	file, err := os.Create(filename)
	defer file.Close()

	if err != nil {
		log.Fatalf("Failed to open %v %v", filename, err)
	}

	file.Write(data)
}

func dumpReport(report *chrome.Report, dumpFilename string) {
	dumpFile(report.Screenshot, dumpFilename, "png")
	dumpFile(report.Content, dumpFilename, "html")
}

func dumpReports(dumpFilename string) {
	reports := &chrome.Reports{}
	err := json.NewDecoder(os.Stdin).Decode(reports)
	if err != nil {
		log.Fatalf("Failed to decode the JSON %v", err)
		return
	}
	if dumpFilename == "" {
		dumpFilename = reports.TransactionID
	}
	if dumpFilename == "" {
		dumpFilename = randomFilename(4)
	}

	for _, report := range reports.URLReports {
		dumpReport(report, dumpFilename)
	}
}

func commandLineMode() bool {

	var parseReport bool
	var dumpFilename string
	var url string
	flag.BoolVar(&parseReport, "parseReport", false, "parse JSON report")
	flag.StringVar(&dumpFilename, "dumpFilename", "", "filename of the dump")
	flag.StringVar(&url, "url", "", "Qucik fetch for a URL, no URL encoding is required")
	flag.Parse()

	if !(parseReport || url != "") {
		return false
	}

	if parseReport {
		dumpReports(dumpFilename)
		return true
	}

	if url != "" {
		browser, err := chrome.New(chrome.BrowserParams{})
		if err != nil {
			log.Fatalf("Failed to start a browste %v %v", err)
		}

		report, err := browser.Report(url, 30*time.Second)
		if err != nil {
			log.Fatalf("Failed to fetch %v %v", url, err)
		}
		reports := &chrome.Reports{
			Count:      1,
			URLReports: []*chrome.Report{report},
			URLs:       []string{url},
		}
		fmt.Print(string(reports.ToJSON(true)))
		return true
	}

	return true
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if commandLineMode() {
		return
	}

	browser, err := chrome.New(chrome.BrowserParams{
		SkipCategory: []string{chrome.WebPageCategoryMedia},
	})

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
