package main

import (
	"strings"
	"testing"
	"time"

	"github.com/pollenjp/cc-pages/internal/config"
)

// TestServeRejectsNonPositiveRescan は --rescan に 0 や負値を渡したときに
// cmdServe がきれいなエラーを返すことを確認する。
//
// time.NewTicker は d <= 0 で panic する契約になっており、Run は goroutine の
// 中で呼ばれるので、検証を怠るとプロセス全体が生のスタックトレースで落ちる。
// この検証は fs.Parse の直後、bind より前に走る必要があるので、ここでは
// cmdServe が (ポート 0 の bind を試みず) 即座に戻ることも確認する。
func TestServeRejectsNonPositiveRescan(t *testing.T) {
	for _, v := range []string{"0", "-1s"} {
		t.Run(v, func(t *testing.T) {
			cfg := config.Config{Addr: "127.0.0.1:0", Root: t.TempDir()}
			done := make(chan error, 1)
			go func() { done <- cmdServe(cfg, []string{"--rescan", v}) }()

			select {
			case err := <-done:
				if err == nil {
					t.Fatalf("--rescan %s がエラーにならなかった", v)
				}
				if !strings.Contains(err.Error(), "--rescan") {
					t.Errorf("エラーメッセージに --rescan が含まれない: %v", err)
				}
			case <-time.After(time.Second):
				// bind まで進んでしまうと ListenAndServe がブロックし続け、
				// ここに来る。検証が bind より前に無いことの徴候。
				t.Fatal("cmdServe が戻らない (bind まで進んでしまった可能性がある)")
			}
		})
	}
}
