package main

import (
	"bytes"
	"flag"
	"github.com/dustin/go-humanize"
	"github.com/jasonlvhit/gocron"
	"github.com/spf13/viper"
	"goed/eddb"
	"goed/edgic"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

type StarStatCfg struct {
	BackupFile   string
	BackupPeriod uint64
}
type EDInfoCenterConf struct {
	EDDBCache   eddb.DataCacheConfig
	CheckPeriod uint64
	GrpcSrv     edgic.GrpcServerConf
	StarStat    StarStatCfg
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
func configLog(logFileName string, silent bool) {
	if silent {
		if len(logFileName) == 0 {
			log.Println("Hmm, silent, no log output.... Standard log will be used")
			return
		}
		logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
		if err == nil {
			log.SetOutput(logFile)
		} else {
			log.Println("Failed to create log file %v", err)
		}
	} else {
		if len(logFileName) > 0 {
			logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
			if err == nil {
				mw := io.MultiWriter(os.Stdout, logFile)
				log.SetOutput(mw)
			} else {
				log.Println("Failed to create log file %v", err)
			}
		} else {
			log.SetOutput(os.Stdout)
			log.Println("Logging to file is disabled")
		}
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile | log.LUTC)
}

func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	log.Printf("Alloc = %s TotalAlloc = %s Sys = %s NumGC = %s Alive objects = %s\n",
		humanize.Bytes(m.Alloc), humanize.Bytes(m.TotalAlloc), humanize.Bytes(m.Sys),
		humanize.Comma(int64(m.NumGC)), humanize.Comma(int64(m.Mallocs-m.Frees)))
}

func main() {
	pprofAddr := flag.String("pprof", "", "host:port for pprof")
	silent := flag.Bool("noout", false, "Exclude stdout from logging")
	logFileName := flag.String("logfile", "edicenter.log", "Log file name. Logging to file will be disabled if empty")

	floodUpdates := flag.Bool("floodUpdates", false, "Flood the memory)). Don't use it))")

	flag.Parse()

	configLog(*logFileName, *silent)

	cfg, err := loadConfig(flag.Arg(0))
	if err != nil {
		log.Fatalf("Failed to read config file %s: %v\n", flag.Arg(0), err)
		return
	}

	if len(*pprofAddr) > 4 {
		go func() {
			log.Printf("Starting pprof at %s\n", *pprofAddr)
			log.Println(http.ListenAndServe(*pprofAddr, nil))
		}()
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
				log.Println("New galaxy info is set")
				printMemUsage()
			} else {
				log.Print("Failed to load initial galaxy info\n")
			}
		}
	}
	gocron.Every(cfg.CheckPeriod).Seconds().Do(checker)
	gocron.Start()

	go ediSrv.Serve()

	eddnListener := eddb.NewShipStatCollector()
	eddnListener.StartListen()

	if *floodUpdates {
		memuser := func() {
			eddbInfo, err := eddb.BuildEDDBInfo(&cfg.EDDBCache)
			if err == nil {
				ediSrv.SetEDDBData(eddbInfo)
				log.Println("New galaxy info is set")
				printMemUsage()
			} else {
				log.Print("Failed to load initial galaxy info\n")
			}
		}
		gocron.Every(10).Seconds().Do(memuser)
	}
	if len(cfg.StarStat.BackupFile) > 0 {
		eddnListener.Restore(cfg.StarStat.BackupFile)
		gocron.Every(60).Seconds().Do(eddnListener.Backup, cfg.StarStat.BackupFile)
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Running. Send me a signal to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	if len(cfg.StarStat.BackupFile) > 0 {
		eddnListener.Backup(cfg.StarStat.BackupFile)
	}

	eddnListener.Shutdown()
}
