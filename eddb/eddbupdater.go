package eddb

import (
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	lastModified = "Last-Modified"
)

type CachedData struct {
	URL       string
	LocalFile string
}

type DataCacheConfig struct {
	Systems     CachedData
	Factions    CachedData
	Stations    CachedData
	Commodities CachedData
	Listings    CachedData
	ProcessListings bool
}

type DataCache struct {
	cfg DataCacheConfig
	tr  *http.Transport
}

func NewDataCache(cfg DataCacheConfig) *DataCache {
	return &DataCache{
		cfg: cfg,
		tr: &http.Transport{
			MaxIdleConns:       2,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		},
	}
}

func (d *CachedData) getRemoteTimestamp(tr *http.Transport) (time.Time, error) {
	client := &http.Client{
		Transport: tr,
		Timeout:   20 * time.Second,
	}
	resp, err := client.Head(d.URL)
	if err != nil {
		return time.Time{}, err
	}

	lm, here := resp.Header[lastModified]
	if !here {
		return time.Time{}, errors.New(fmt.Sprintf("Missing %s header on %s", lastModified, d.URL))
	}
	if len(lm) != 1 {
		if len(lm) == 0 {
			return time.Time{}, errors.New(fmt.Sprintf("Missing %s header on %s", lastModified, d.URL))
		} else {
			log.Printf("Warn: multiple %s header on %s\n", lastModified, d.URL)
		}

	}
	return time.Parse(http.TimeFormat, lm[0])
}

func (d *CachedData) getLocalFileTimestamp() (time.Time, error) {
	statInfo, err := os.Stat(d.LocalFile)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil
		}
		log.Printf("Error stating file %s %v\n", d.LocalFile, err)
		return time.Time{}, err
	}
	return statInfo.ModTime(), nil
}

func (d *CachedData) needUpdate(tr *http.Transport) (bool, error) {
	remoteTime, err := d.getRemoteTimestamp(tr)
	if err != nil {
		return false, errors.New(fmt.Sprintf("Error reading time %s %v", d.URL, err))
	}
	log.Printf("Remote %s touched %s\n", d.URL, humanize.Time(remoteTime))
	localTime, err := d.getLocalFileTimestamp()
	if err != nil {
		log.Printf("Got an error on localfile %v", err)
	}

	return remoteTime.After(localTime), nil
}

// WriteCounter counts the number of bytes written to it. It implements to the io.Writer
// interface and we can pass this into io.TeeReader() which will report progress on each
// write cycle.
type WriteCounter struct {
	Total         uint64
	LastPrintTime time.Time
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	now := time.Now()
	if now.Sub(wc.LastPrintTime).Seconds() > 10 {
		wc.PrintProgress()
		wc.LastPrintTime = now
	}
	return n, nil
}

func (wc WriteCounter) PrintProgress() {
	log.Printf("Downloading... %s complete\n", humanize.Bytes(wc.Total))
}

func (d *CachedData) downloadUpdate(tr *http.Transport) error {
	tmpPath := d.LocalFile + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		log.Printf("Create %s failed: %v\n", d.LocalFile, err)
		return err
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   20 * time.Second,
	}
	resp, err := client.Get(d.URL)
	if err != nil {
		log.Printf("Download failed on %s: %v\n", d.URL, err)
		out.Close()
		return err
	}
	// Create our progress reporter and pass it to be used alongside our writer
	counter := &WriteCounter{}

	_, err = io.Copy(out, io.TeeReader(resp.Body, counter))
	resp.Body.Close()
	out.Close()

	if err != nil {
		return err
	}

	err = os.Remove(d.LocalFile)
	if err != nil {
		log.Printf("Remove failed: %v (ignored)\n", err)
	}

	err = os.Rename(tmpPath, d.LocalFile)
	if err != nil {
		log.Printf("Rename %s -> %s failed: %v\n", tmpPath, d.LocalFile, err)
		return err
	}
	return nil
}

func (dc *DataCache) CheckForUpdates() ([]*CachedData, error) {
	rv := make([]*CachedData, 0)

	//	idx := 0
	for _, item := range []*CachedData{&dc.cfg.Systems,
		&dc.cfg.Stations,
		&dc.cfg.Factions,
		&dc.cfg.Commodities,
		&dc.cfg.Listings} {
		tbu, err := item.needUpdate(dc.tr)
		if err != nil {
			return nil, err
		}
		if tbu {
			rv = append(rv, item)
		}
	}

	for _, d := range rv {
		log.Printf("Update wanted on %s -> %s \n", d.URL, d.LocalFile)
		d.downloadUpdate(dc.tr)
	}

	return rv, nil
}
