package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/samuel/go-zookeeper/zk"
	"github.com/uber-go/zap"
)

var logger = zap.New(zap.NewTextEncoder())
var contents = []byte("hello")

func updateNodes(stopchan chan int, tickerChan <-chan time.Time, conn *zk.Conn, nodes []string) {
	for {
		select {
		case <-tickerChan:
			logger.Info("ticker tick")
			for _, node := range nodes {
				logger.Info("creating node", zap.String("node", node), zap.Object("content", contents))
				conn.Create(node, contents, 0 /*flags */, zk.WorldACL(0x1f))
			}
		case <-stopchan:
			// stop
			logger.Info("stopping node routine")
			return
		}
	}
}

func handleCtrlC(c chan os.Signal, quit chan int) {
	sig := <-c
	// handle ctrl+c event here
	// for example, close database
	fmt.Println("\nsignal: ", sig)
	quit <- 1 // stop other routines
	os.Exit(0)
}

func main() {
	quit := make(chan int)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	conn, _, err := zk.Connect([]string{"127.0.0.1"}, time.Second)
	if err != nil {
		panic(err)
	}

	nodes := []string{
		"/foo",
		"/bar",
	}
	ticker := time.NewTicker(time.Second * 1).C

	go updateNodes(quit, ticker, conn, nodes)
	go handleCtrlC(c, quit)

	select {}
}
