// Package store は cc-pages のディスク上の形式を実装する唯一の場所。
//
// ディレクトリレイアウト・page.json / session.json のスキーマ・採番規則は
// すべてこのパッケージに閉じる。skill も server もここを通してしかディスクに触らない。
package store

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// slugMaxRunes はスラッグの最大長 (rune 単位)。
const slugMaxRunes = 40

// Slug はタイトルをディレクトリ名として安全な形に落とす。
//
// 文字と数字はそのまま残し (日本語も残る)、それ以外は "-" に潰す。連続する "-" は
// 1 つにまとめ、前後の "-" を削り、40 rune で切る。残る文字が無ければ空文字を返す。
func Slug(title string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteRune('-')
			prevDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if r := []rune(s); len(r) > slugMaxRunes {
		s = strings.Trim(string(r[:slugMaxRunes]), "-")
	}
	return s
}

// FormatID は連番を 4 桁ゼロ埋めの文字列にする。4 桁を超えたらそのまま伸ばす。
func FormatID(n int) string { return fmt.Sprintf("%04d", n) }

// SessionDirName は "<YYYYMMDD>-<session id の先頭 8 桁>" を返す。
//
// 日付を前置するのは、ls やファイルマネージャで時系列に並ぶようにするため。
func SessionDirName(createdAt time.Time, sessionID string) string {
	short := sessionID
	if len(short) > 8 {
		short = short[:8]
	}
	return createdAt.Format("20060102") + "-" + short
}

// PageDirName は "<連番>-<スラッグ>" を返す。スラッグが空なら連番のみ。
func PageDirName(id, slug string) string {
	if slug == "" {
		return id
	}
	return id + "-" + slug
}
