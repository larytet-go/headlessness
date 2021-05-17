package chrome

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

type AdBlockIfc interface {
	IsAd(string) bool
}

type AdBlockDummy struct {
}

func (ab *AdBlockDummy) IsAd(_ string) bool {
	return false
}

type adBlockList struct {
	blockList map[string]struct{}
}

func NewAdBlockList(filenames []string) (AdBlockIfc, error) {
	adBlockList := &adBlockList{}
	err := adBlockList.load(filenames)
	if err != nil {
		return &AdBlockDummy{}, err
	}
	return adBlockList, nil
}

func (ab *adBlockList) load(filenames []string) error {
	blockList := map[string]struct{}{}
	for _, filename := range filenames {
		count := 0
		file, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("Failed to open %s %v", filename, err)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			s := scanner.Text()
			columns := strings.Split(s, " ")
			if len(columns) < 2 {
				continue
			}
			ip := strings.TrimSpace(columns[0])
			if ip != "0.0.0.0" {
				continue
			}
			blockList[strings.TrimSpace(columns[1])] = struct{}{}
			count++
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("Failed to read %s %v", filename, err)
		}
		log.Printf("AdBlock loaded %d hosts from %s", count, filename)
	}
	return nil
}

func getTLD(hostname string, count int) string {
	words := strings.Split(hostname, ".")
	if len(words) > count {
		words = words[len(words)-count-1:]
		return strings.Join(words[:], ".")
	}

	return hostname
}

func (ab *adBlockList) IsAd(hostname string) bool {
	if _, ok := ab.blockList[hostname]; ok {
		return true
	}
	if _, ok := ab.blockList[getTLD(hostname, 2)]; ok {
		return true
	}
	if _, ok := ab.blockList[getTLD(hostname, 3)]; ok {
		return true
	}
	return false
}
