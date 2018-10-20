package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/spf13/viper"
	"goed/cyborg"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
)

type CyborgBotConfig struct {
	DiscordConf cyborg.CyborgBotDiscordConfig
	GalaxyInfoCenter struct {
		Address string
	}
}

func (c *CyborgBotConfig) check() error {
	return c.DiscordConf.CheckConfig()
}

func loadConfig(path string) (*CyborgBotConfig, error) {
	var cfg CyborgBotConfig

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
	err = cfg.check()
	return &cfg, err
}

func main() {
	logFile, err := os.OpenFile("log.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err == nil {
		mw := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(mw)
	} else {
		fmt.Println("Failer to create log file %v", err)
	}

	debug := flag.Bool("debug", false, "switch on debuging mode")
	silent := flag.Bool("silent", false, "be silent")
	pprofAddr := flag.String("pprof", "", "host:port for pprof")

	flag.Parse()
	loglevel := discordgo.LogInformational
	if *debug {
		loglevel = discordgo.LogDebug
	}
	if *silent {
		loglevel = discordgo.LogWarning
	}

	cfg, err := loadConfig(flag.Arg(0))
	if err != nil {
		log.Fatalf("Failed to read config file %s: %v\n", flag.Arg(0), err)
		return
	}

	bot := cyborg.NewCybordBot(&cfg.DiscordConf, cfg.GalaxyInfoCenter.Address)
	err = bot.Connect(loglevel)

	if err != nil {
		fmt.Println("error creating bot,", err)
		return
	}

	if len(*pprofAddr) > 4 {
		go func() {
			log.Println(http.ListenAndServe(*pprofAddr, nil))
		}()
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	bot.Close()
}
