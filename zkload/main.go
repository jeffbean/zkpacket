package main

import (
	"flag"
	"fmt"
	"math/rand"
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

type znode struct{ path string }

func (z *znode) String() string {
	return fmt.Sprintf("%v", z.path)
}

func init() {
	flag.StringVar(&zkHost, "zk-host", "127.0.0.1", "Host address of zookeeper ensemble")
}

func updateNodes(stopchan chan int, conn *zk.Conn, tickerChan <-chan time.Time) {
	// Create and seed the generator.
	// Typically a non-fixed seed should be used, such as time.Now().UnixNano().
	// Using a fixed seed will produce the same output on every run.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		select {
		case <-tickerChan:
			logger.Info("ticker tick", zap.Int64("conn", conn.SessionID()))
			node := &znode{fmt.Sprintf("/node-%v", r.Int31())}
			_, err := conn.Create(node.String(), contents, 1 /*flags */, zk.WorldACL(0x1f))
			if err != nil {
				panic(err)
			}
			_, _, err = conn.Get(node.String())
			if err != nil {
				logger.Errorf("failed to get node: %#v", node)
				stopchan <- 1
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
	logger, _ := zap.NewDevelopmentConfig().Build()
	sugar = logger.Sugar()

	quit := make(chan int)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	flag.Parse()

	conn, _, err := zk.Connect([]string{zkHost}, time.Second)
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(time.Millisecond * 50).C

	go updateNodes(quit, conn, ticker)
	go handleCtrlC(c, quit)

	select {}
}
