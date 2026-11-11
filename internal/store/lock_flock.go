//go:build linux || darwin

package store

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// lockFile は path を flock で排他に lock し、外す関数を返す。
//
// 待ち方は LOCK_NB を短い間隔で試し直す形にして、timeout で諦める。cc-pages link は
// turn の終わりごとに打たれるので、lock を握ったまま止まった相手がいても、こちらまで
// 止まり続けてはいけない (flock は握った process が落ちれば OS が外す)。
func lockFile(path string, timeout time.Duration) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	fd := int(f.Fd())
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				_ = syscall.Flock(fd, syscall.LOCK_UN)
				_ = f.Close()
			}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR) {
			_ = f.Close()
			return nil, fmt.Errorf("%s を lock できない: %w", path, err)
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("%s の lock が %v のあいだ取れない", path, timeout)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
