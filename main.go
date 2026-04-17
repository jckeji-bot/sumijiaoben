package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	targetURL = "https://www.binance.com/zh-CN/activity/trading-competition/spot-altcoin-festival-wave-XAUt"
	dbPath    = "leaderboard.db"
	interval  = time.Minute
)

func main() {
	viewMode := flag.Bool("view", false, "打印最新快照并退出")
	watchMode := flag.Bool("watch", false, "持续刷新排行榜显示（每10秒）")
	flag.Parse()

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := initSchema(db); err != nil {
		log.Fatalf("init schema: %v", err)
	}

	if *viewMode {
		if err := printLatestSnapshot(db); err != nil {
			log.Fatalf("view: %v", err)
		}
		return
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *watchMode {
		watchTicker := time.NewTicker(10 * time.Second)
		defer watchTicker.Stop()
		clearScreen()
		if err := printLatestSnapshot(db); err != nil {
			log.Printf("[WARN] dashboard: %v", err)
		}
		for {
			select {
			case <-watchTicker.C:
				clearScreen()
				if err := printLatestSnapshot(db); err != nil {
					log.Printf("[WARN] dashboard: %v", err)
				}
			case <-sigCtx.Done():
				return
			}
		}
	}

	run := func() {
		entries, err := scrapeLeaderboard()
		if err != nil {
			log.Printf("[WARN] scrape failed: %v", err)
			return
		}
		if len(entries) == 0 {
			log.Println("[WARN] 0 entries parsed, skipping save")
			return
		}
		if err := saveSnapshot(db, entries); err != nil {
			log.Printf("[ERROR] save snapshot: %v", err)
			return
		}
		log.Printf("[OK] saved %d entries at %s", len(entries), time.Now().Format(time.RFC3339))
	}

	run() // first run immediately

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			run()
		case <-sigCtx.Done():
			log.Println("shutting down")
			return
		}
	}
}
