package main

import (
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"io"
	"log"
	"os"
	"os/signal"
	"goed/pkg/cyborg"
	"syscall"
)



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

	flag.Parse()
	loglevel := discordgo.LogInformational
	if *debug {
		loglevel = discordgo.LogDebug
	}
	if *silent {
		loglevel = discordgo.LogWarning
	}

	cfg, err := cyborg.LoadConfig(flag.Arg(0))
	if err != nil {
		log.Fatalf("Failed to read config file %s: %v\n", flag.Arg(0), err)
		return
	}

	bot := cyborg.NewCybordBot(cfg)
	err = bot.Connect(loglevel)

	if err != nil {
		fmt.Println("error creating bot,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	bot.Close()
}
