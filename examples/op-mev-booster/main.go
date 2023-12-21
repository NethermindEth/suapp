package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
)

const (
	LogLevel          = "debug"
	DefaultListenAddr = "localhost:18550"
)

func main() {
	log := logrus.NewEntry(logrus.New())
	log.Logger.SetOutput(os.Stdout)

	lvl, err := logrus.ParseLevel(LogLevel)
	if err != nil {
		flag.Usage()
		log.Fatalf("invalid loglevel: %s", LogLevel)
	}
	log.Logger.SetLevel(lvl)

	evListSrv, err := NewEventListener(log)
	if err != nil {
		log.WithError(err).Fatal("failed creating the event listener")
	}

	httpSrv, err := NewBoostService(log, DefaultListenAddr)
	if err != nil {
		log.WithError(err).Fatal("failed creating the server")
	}

	go evListSrv.Listen()

	log.Println("listening on", DefaultListenAddr)
	log.Fatal(httpSrv.StartHTTPServer())
}
