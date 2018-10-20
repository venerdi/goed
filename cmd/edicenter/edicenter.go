package main

import (
	"bytes"
	"flag"
	"github.com/jasonlvhit/gocron"
	"github.com/spf13/viper"
	"goed/edgic"
	"goed/eddb"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type EDInfoCenterConf struct {
	EDDBCache   eddb.DataCacheConfig
	CheckPeriod uint64
	GrpcSrv     edgic.GrpcServerConf
}

func loadConfig(path string) (*EDInfoCenterConf, error) {
	var cfg EDInfoCenterConf

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	viper.SetConfigType("yaml")

	err = viper.ReadConfig(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	viper.Unmarshal(&cfg)
	//	err = cfg.check()
	return &cfg, err
}
func main() {
	logFile, err := os.OpenFile("edicenter.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err == nil {
		mw := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(mw)
	} else {
		log.Println("Failer to create log file %v", err)
	}

	flag.Parse()
	cfg, err := loadConfig(flag.Arg(0))
	if err != nil {
		log.Fatalf("Failed to read config file %s: %v\n", flag.Arg(0), err)
		return
	}
	dc := eddb.NewDataCache(cfg.EDDBCache)
	dc.CheckForUpdates()

	ediSrv := edgic.NewGIServer(cfg.GrpcSrv)
	eddbInfo, err := eddb.BuildEDDBInfo(&cfg.EDDBCache)
	if err == nil {
		ediSrv.SetEDDBData(eddbInfo)
	} else {
		log.Print("Failed to load initial galaxy info\n")
	}

	checker := func() {
		updates, err := dc.CheckForUpdates()
		if err != nil {
			log.Printf("EDDB cahce update failed: %v\n", err)
			return
		}
		if updates != nil && len(updates) > 0 {
			eddbInfo, err := eddb.BuildEDDBInfo(&cfg.EDDBCache)
			if err == nil {
				ediSrv.SetEDDBData(eddbInfo)
			} else {
				log.Print("Failed to load initial galaxy info\n")
			}

		}
	}
	gocron.Every(cfg.CheckPeriod).Seconds().Do(checker)
	gocron.Start()
	
	go ediSrv.Serve()

	// Wait here until CTRL-C or other term signal is received.
	log.Println("EdInfoCenter is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

}
