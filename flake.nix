{
  description = "cc-pages の e2e をローカルで回すための devShell (Node + Chromium)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    { nixpkgs, ... }:
    let
      inherit (nixpkgs) lib;

      # chromium は nixpkgs では Linux のみのため darwin は含めない
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllSystems = lib.genAttrs systems;
    in
    {
      formatter = forAllSystems (system: nixpkgs.legacyPackages.${system}.nixfmt);

      # GC ルートを作って再ダウンロードを防ぎたいとき用:
      #   nix build .#chromium -o .cache/playwright-chromium
      packages = forAllSystems (system: {
        chromium = nixpkgs.legacyPackages.${system}.chromium;
      });

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};

          # Chromium はフォントを同梱せず fontconfig を見る。素の WSL2 には
          # 日本語フォントが 1 つも無く、CJK グリフが豆腐 (□) になる。
          # assertion は DOM を見るので通ってしまい、スクショだけが無価値になる
          # ため、ここは飾りではない。makeFontsConf は実環境のフォントを
          # 残したまま追加するので、ホストの設定を奪わない。
          fontsConf = pkgs.makeFontsConf {
            fontDirectories = [
              pkgs.noto-fonts-cjk-sans # 日本語グリフ
              pkgs.noto-fonts-color-emoji # 絵文字
              pkgs.liberation_ttf # Arial / Times 等のメトリック互換
            ];
          };
        in
        {
          default = pkgs.mkShellNoCC {
            # Go は入れない。この repo の Go は mise.toml が pin しており
            # (go = "1.27")、devShell 側にも Go を置くと PATH の先頭を奪って
            # 版が食い違う。実際 `compile: version "go1.27.1" does not match
            # go tool version "go1.26.7"` で webServer が起動しなくなった。
            # ここが供給するのは「ブラウザ側に足りないもの」だけ。
            packages = [
              pkgs.chromium
              pkgs.nodejs_24
            ];
            # playwright.config.ts が CHROMIUM_BIN を読む。空だと (CI 以外では)
            # 分かる形で止まる。CI は公式 image 同梱のブラウザを使うので空でよい。
            shellHook = ''
              export CHROMIUM_BIN="${pkgs.chromium}/bin/chromium"
              export FONTCONFIG_FILE="${fontsConf}"
            '';
          };
        }
      );
    };
}
