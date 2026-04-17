package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"
	"time"
)

func printLatestSnapshot(db *sql.DB) error {
	var latestAt string
	err := db.QueryRow(
		`SELECT captured_at FROM snapshots ORDER BY captured_at DESC LIMIT 1`,
	).Scan(&latestAt)
	if err == sql.ErrNoRows {
		fmt.Println("数据库暂无数据，请先运行抓取程序。")
		return nil
	}
	if err != nil {
		return err
	}

	t, _ := time.Parse(time.RFC3339, latestAt)

	var totalSnapshots int
	_ = db.QueryRow(`SELECT COUNT(DISTINCT captured_at) FROM snapshots`).Scan(&totalSnapshots)

	var totalEntries int
	_ = db.QueryRow(`SELECT COUNT(*) FROM snapshots WHERE captured_at = ?`, latestAt).Scan(&totalEntries)

	fmt.Println("┌─────────────────────────────────────────────────────────┐")
	fmt.Println("│           Binance 排行榜 · 实时数据库快照                    │")
	fmt.Println("├─────────────────────────────────────────────────────────┤")
	fmt.Printf("│  抓取时间  : %-44s│\n", t.Local().Format("2006-01-02 15:04:05"))
	fmt.Printf("│  本次条目数: %-44d│\n", totalEntries)
	fmt.Printf("│  历史快照数: %-44d│\n", totalSnapshots)
	fmt.Println("└─────────────────────────────────────────────────────────┘")
	fmt.Println()

	rows, err := db.Query(
		`SELECT rank, COALESCE(user_id, ''), username, volume FROM snapshots
		 WHERE captured_at = ?
		 ORDER BY rank ASC`,
		latestAt,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "排名\tUserID\t用户名\t交易量\t")
	fmt.Fprintln(w, "----\t------\t------\t------\t")
	for rows.Next() {
		var rank int
		var userID, username, volume string
		if err := rows.Scan(&rank, &userID, &username, &volume); err != nil {
			return err
		}
		fmt.Fprintf(w, "#%d\t%s\t%s\t%s\t\n", rank, userID, username, volume)
	}
	w.Flush()

	fmt.Printf("\n  [界面刷新于 %s，按 Ctrl+C 退出]\n", time.Now().Local().Format("15:04:05"))
	return rows.Err()
}

func clearScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}
