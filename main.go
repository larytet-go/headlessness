package main

import (
	"encoding/base64"
	"encoding/binary"
	"flag"
	"log"
	"net/http"
	"time"

	"headlessness/chrome"
)

// Based on https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go/22892986#22892986
var filenameLetters = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randomFilename(l int) string {
	lenFilenameLetters := len(filenameLetters)
	runes := make([]rune, l)
	for i := range runes {
		runes[i] = IDLetters[rand.Intn(lenFilenameLetters)]
	}
	return string(runes)
}

func dumpFile(base64String, dumpFilename, extension string) {
	data := []byte{}
	_, err := base64.StdEncoding.Decode(data, []byte(base64String))
	if err != nil {
		log.Fatalf("Failed to decode %v %v", extension, err)
		return
	}

	filename := dumpFilename + "." + randomFilename(4) + extension
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
	reports := &Reports{}
	err := json.NewDecoder(os.Stdin).Decode(reportPayload)
	if err != nil {
		fmt.Print(err.Erorr())
		return
	}
	if dumpFilename == "" {
		dumpFilename = reports.TransactionID
	}
	if dumpFilename == "" {
		dumpFilename = randomFilename(4)
	}

	for _, report := range reports.URLReports {
		dumpReport(&report, dumpFilename)
	}
}

func main() {
	var parseReport bool
	var dumpFilename string
	flag.IntVar(&parseReport, "parseReport", false, "parse JSON report")
	flag.IntVar(&dumpFilename, "dumpFilename", "", "filename of the dump")
	flag.Parse()
	if parseReport != "" {
		dumpReports(dumpFilename)
		return
	}

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
