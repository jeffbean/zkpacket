package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grafana/grafana/pkg/cmd/grafana-cli/logger"
	"github.com/jeffbean/go-zookeeper/zk"
	"go.uber.org/zap"
)

var (
	sugar    *zap.SugaredLogger
	contents = []byte("hello")
	zkHost   string
)

func init() {
	flag.StringVar(&zkHost, "zk-host", "127.0.0.1", "Host address of zookeeper ensemble")
}

func updateNodes(stopchan chan int, tickerChan <-chan time.Time, conn *zk.Conn, nodes []string) {
	for {
		select {
		case <-tickerChan:
			logger.Info("ticker tick", zap.Int64("conn", conn.SessionID()))
			for _, node := range nodes {
				sugar.Info("creating node", "node", node, "content", contents)
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
	logger, _ := zap.NewDevelopment()
	sugar = logger.Sugar()

	quit := make(chan int)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	flag.Parse()

	conn, _, err := zk.Connect([]string{zkHost}, time.Second)
	if err != nil {
		panic(err)
	}
	conn2, _, err := zk.Connect([]string{zkHost}, time.Second)
	if err != nil {
		panic(err)
	}

	nodes := []string{
		"/foo",
		"/bar",
	}
	ticker := time.NewTicker(time.Second * 10).C
	ticker2 := time.NewTicker(time.Second * 15).C
	go updateNodes(quit, ticker, conn, nodes)
	go updateNodes(quit, ticker2, conn2, nodes)
	go handleCtrlC(c, quit)

	select {}
}
