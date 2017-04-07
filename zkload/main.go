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
	"go.uber.org/zap/zapcore"
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
	flag.StringVar(&frequency, "frequency", "10s", "How often to run a bunch of actions on a znode")
	flag.Int64Var(&randSeed, "seed", time.Now().UnixNano(), "Optional seeded int64 for the randomness")
}

func printWatchEvents(eventChan <-chan zk.Event) {
	select {
	case ev := <-eventChan:
		if ev.Err != nil {
			logger.Error("event error", zap.Error(ev.Err))
		}
		logger.Info("EVENT!", zap.Any("event", ev))
	}
}

func updateNodes(stopchan chan int, r *rand.Rand, conn *zk.Conn, tickerChan <-chan time.Time) {

	for {
		select {
		case <-tickerChan:
			// logger.Debug("ticker tick", zap.Int64("conn", conn.SessionID()))
			node := &znode{fmt.Sprintf("/node-%v", r.Int31())}
			if _, err := conn.Create(node.String(), contents, 1 /*flags */, zk.WorldACL(0x1f)); err != nil {
				logger.Error("failed to create node", zap.Error(err), zap.Stringer("node", node))
			}
			_, _, eventChan, err := conn.GetW(node.String())
			if err != nil {
				logger.Error("failed to GetW", zap.Stringer("node", node), zap.Error(err))
			}
			go printWatchEvents(eventChan)
			if _, _, err := conn.Get(node.String()); err != nil {
				logger.Error("failed to get", zap.Stringer("node", node), zap.Error(err))
			}
			if _, _, err := conn.Exists(node.String()); err != nil {
				logger.Error("failed to Exists", zap.Stringer("node", node), zap.Error(err))
			}
			if _, _, err := conn.GetACL(node.String()); err != nil {
				logger.Error("failed to get ACL", zap.Stringer("node", node), zap.Error(err))
			}
			if _, err := conn.SetACL(node.String(), zk.WorldACL(0x1f), 0 /* version */); err != nil {
				logger.Error("failed to set ACL", zap.Stringer("node", node), zap.Error(err))
			}
			if _, _, err := conn.GetACL(node.String()); err != nil {
				logger.Error("failed to get ACL", zap.Stringer("node", node), zap.Error(err))
			}
			if _, _, err := conn.Children(node.String()); err != nil {
				logger.Error("failed to get children", zap.Stringer("node", node), zap.Error(err))
			}
			if _, err := conn.Set(node.String(), []byte("i want to set this now"), -1 /* version */); err != nil {
				logger.Error("failed to Set", zap.Stringer("node", node), zap.Error(err))
			}
			_, _, _, err = conn.GetW(node.String())
			if err != nil {
				logger.Error("failed to GetW", zap.Stringer("node", node), zap.Error(err))
			}
			multiNode := &znode{fmt.Sprintf("/multinode-%v", r.Int31())}
			ops := []interface{}{
				&zk.CreateRequest{Path: multiNode.String(), Data: []byte{1, 2, 3, 4}, Acl: zk.WorldACL(zk.PermAll)},
				&zk.SetDataRequest{Path: multiNode.String(), Data: []byte{1, 2, 3, 4, 5}, Version: -1},
			}
			if res, err := conn.Multi(ops...); err != nil {
				logger.Error("Multi returned error", zap.Error(err), zap.Stringer("node", node))
			} else if len(res) != 2 {
				logger.Error("Expected 2 responses", zap.Int("actual", len(res)))
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
	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.EncoderConfig = zapcore.EncoderConfig{
		LevelKey:      "L",
		TimeKey:       "",
		MessageKey:    "M",
		NameKey:       "N",
		CallerKey:     "C",
		StacktraceKey: "S",
		EncodeLevel:   zapcore.CapitalColorLevelEncoder,
	}
	logger, _ = loggerConfig.Build()

	quit := make(chan int)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	flag.Parse()
	freq, err := time.ParseDuration(frequency)
	if err != nil {
		logger.Fatal("failed to parse frequency duration")
	}

	conn, _, err := zk.Connect([]string{zkHost}, 3*time.Second)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Create and seed the generator.
	// Typically a non-fixed seed should be used, such as time.Now().UnixNano().
	// Using a fixed seed will produce the same output on every run.
	r := rand.New(rand.NewSource(randSeed))

	ticker := time.Tick(freq)
	// r2 := rand.New(rand.NewSource(time.Now().UnixNano()))
	// ticker2 := time.Tick(freq)
	// go updateNodes(quit, r2, conn, ticker2)
	time.Sleep(5 * time.Second)
	go updateNodes(quit, r, conn, ticker)

	go handleCtrlC(c, quit)

	select {}
}
