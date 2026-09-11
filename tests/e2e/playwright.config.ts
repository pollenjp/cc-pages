import { defineConfig } from '@playwright/test';
import * as path from 'node:path';

// サーバを誰が起こすかを環境で分ける。
//
// ローカル: webServer が `go run . serve` を起こす。
// CI:       テストは公式 playwright image の中で走り、その中に Go は無い。
//           host 側で先に起こしてあるので E2E_BASE_URL を渡してもらい、
//           webServer は使わない。ここを分けないと CI で必ず落ちる。
const external = process.env.E2E_BASE_URL;

const PORT = 7788;
const baseURL = external ?? `http://127.0.0.1:${PORT}`;

const repoRoot = path.resolve(__dirname, '..', '..');
const fixtureRoot = path.join(__dirname, 'fixtures', 'root');

// ローカルでブラウザを供給するのは nix の devShell (flake.nix)。
// CI では image 同梱のブラウザを使うので CHROMIUM_BIN は空でよい。
// devShell の外でうっかり叩いたときだけ、分かる形で止める。
if (!external && !process.env.CI && !process.env.CHROMIUM_BIN) {
  throw new Error(
    'CHROMIUM_BIN が空です。`nix develop --command npx playwright test` で実行してください\n' +
      '(flake.nix の devShell が CHROMIUM_BIN と FONTCONFIG_FILE を export します)',
  );
}

export default defineConfig({
  testDir: __dirname,
  outputDir: path.join(__dirname, 'test-results'),
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['list']] : [['list']],

  use: {
    baseURL,
    trace: 'retain-on-failure',
    // 幅を固定しないとスクショの折り返しが環境で変わり、レビューで差分に見える
    viewport: { width: 1100, height: 900 },
    ...(process.env.CHROMIUM_BIN
      ? { launchOptions: { executablePath: process.env.CHROMIUM_BIN } }
      : {}),
  },

  webServer: external
    ? undefined
    : {
        command: 'go run . serve',
        cwd: repoRoot,
        url: `${baseURL}/`,
        // 既に 7788 で何かが動いていたら、それはユーザーの実データを配っている
        // サーバかもしれない。使い回さずに落とす。
        reuseExistingServer: false,
        stdout: 'pipe',
        stderr: 'pipe',
        env: {
          CC_PAGES_ROOT: fixtureRoot,
          CC_PAGES_ADDR: `127.0.0.1:${PORT}`,
        },
      },
});
