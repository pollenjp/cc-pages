//go:build !linux && !darwin

package store

import "time"

// lockFile は linux・darwin 以外では lock しない。
//
// cc-pages を使うのは linux (WSL を含む) と macOS で、ほかの OS では build が通ることだけを
// 保つ。この OS で同じ会話に cc-pages link が同時に来ると、片方の書き込みが消えうる。
func lockFile(path string, timeout time.Duration) (func(), error) {
	return func() {}, nil
}
