package store

import (
	"os"
	"path/filepath"
)

// writeFileAtomic は path を、同じディレクトリの一時ファイルに書いてから rename で
// 置き換える。
//
// os.WriteFile は既存のファイルを切り詰めてから書くので、その間に読んだ側 (serve の
// 走査や、別の cc-pages の findOrMakeSessionDir) は空や書きかけの JSON を見る。
// rename は同じファイルシステムの中なら一瞬で入れ替わるので、読む側は前の中身か
// 新しい中身のどちらかしか見ない。
//
// 一時ファイルの名前は "." で始める。Scan はセッション直下のディレクトリだけを
// ページとして読むので、途中で落ちて一時ファイルが残っても一覧には出ない。
func writeFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	dir, base := filepath.Split(path)
	f, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Chmod(perm); err != nil {
		_ = f.Close()
		return err
	}
	// rename の前に中身を落とす。落とさずに rename すると、電源断のあとに
	// 名前だけ新しく中身が空のファイルが残ることがある。
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
