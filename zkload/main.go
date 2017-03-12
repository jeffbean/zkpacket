package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jeffbean/go-zookeeper/zk"
	"go.uber.org/zap"
)

var (
	logger    *zap.Logger
	contents  = []byte("hello")
	zkHost    string
	frequency string
	randSeed  int64
)

type znode struct{ path string }

func (z *znode) String() string {
	return fmt.Sprintf("%v", z.path)
}

func init() {
	flag.StringVar(&zkHost, "zk-host", "127.0.0.1", "Host address of zookeeper ensemble")
	flag.StringVar(&frequency, "frequency", "1s", "How often to run a bunch of actions on a znode")
	flag.Int64Var(&randSeed, "seed", time.Now().UnixNano(), "Optional seeded int64 for the randomness")
}

func updateNodes(stopchan chan int, r *rand.Rand, conn *zk.Conn, tickerChan <-chan time.Time) {

	for {
		select {
		case <-tickerChan:
			logger.Debug("ticker tick", zap.Int64("conn", conn.SessionID()))
			node := &znode{fmt.Sprintf("/node-%v", r.Int31())}
			_, err := conn.Create(node.String(), contents, 1 /*flags */, zk.WorldACL(0x1f))
			if err != nil {
				panic(err)
			}
			_, _, err = conn.Get(node.String())
			if err != nil {
				logger.Error("failed to get node", zap.Stringer("node", node))
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
	logger, _ = zap.NewDevelopmentConfig().Build()

	quit := make(chan int)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	flag.Parse()
	freq, err := time.ParseDuration(frequency)
	if err != nil {
		logger.Fatal("failed to parse frequency duration")
	}

	conn, _, err := zk.Connect([]string{zkHost}, time.Second)
	if err != nil {
		panic(err)
	}

	// Create and seed the generator.
	// Typically a non-fixed seed should be used, such as time.Now().UnixNano().
	// Using a fixed seed will produce the same output on every run.
	r := rand.New(rand.NewSource(randSeed))

	ticker := time.Tick(freq)
	// r2 := rand.New(rand.NewSource(time.Now().UnixNano()))
	// ticker2 := time.Tick(freq)
	// go updateNodes(quit, r2, conn, ticker2)

	go updateNodes(quit, r, conn, ticker)

	go handleCtrlC(c, quit)

	select {}
}
