package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
)

const (
	LogLevel          = "debug"
	DefaultListenAddr = "0.0.0.0:18585"
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

	httpSrv, err := NewBoostService(log, DefaultListenAddr)
	if err != nil {
		log.WithError(err).Fatal("failed creating the server")
	}

	log.Println("listening on", DefaultListenAddr)
	log.Fatal(httpSrv.StartHTTPServer())
}
