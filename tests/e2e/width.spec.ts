import { test, expect, type Page } from '@playwright/test';
import * as path from 'node:path';

// 本文幅トグル (標準 52rem ⇄ 全幅) の e2e。
//
// 撮っていないもの (撮れないものではなく、意図的に範囲外):
// - 一覧 / セッションのページ。トグルは chrome にあるので同じように効くが、
//   幅の違いを絵で見るなら図のあるページ 1 枚で足りる
// - standalone。chrome が被らないのでトグルも出ない。Go 側の
//   TestStandalonePageHasNoChrome が字面で見ている
// - ダークモード。幅の話に配色は効かない (配色は diff.spec.ts が撮る)

const PAGE_URL = '/p/20260101-e2efixture/' + encodeURIComponent('0002-横に広い図') + '/';
const SHOTS = path.join(__dirname, 'screenshots');

/** 図がその枠に収まらず、横スクロールが要る状態か。 */
async function figureOverflows(page: Page): Promise<boolean> {
  return page.locator('#wide-figure').evaluate((box) => {
    const img = box.querySelector('img') as HTMLElement;
    // 1px 未満の差は小数丸めなので溢れと呼ばない
    return img.getBoundingClientRect().width - box.clientWidth > 1;
  });
}

/** 本文枠 (.wrap) の実際の幅と、ページが使える幅。 */
async function widths(page: Page): Promise<{ wrap: number; viewport: number }> {
  return page.evaluate(() => ({
    wrap: document.querySelector('main.wrap')!.getBoundingClientRect().width,
    // scrollbar を除いた、レイアウトが使える幅
    viewport: document.documentElement.clientWidth,
  }));
}

/** トグルのラベルのうち、CSS で表示されている方。
 *
 * toHaveText は textContent を見るので、この button では隠れている側の span も
 * 拾って "全幅にする標準幅にする" になる。:visible で絞ってから読む。
 */
async function toggleLabel(page: Page): Promise<string> {
  return page.locator('.widthtoggle span:visible').innerText();
}

test('標準幅では図が本文枠に収まらない', async ({ page }) => {
  await page.goto(PAGE_URL);
  // 真っ白でも通る「撮るだけの test」にしない。このページ固有の見出しを見る
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('横に広い図');

  // 既定は標準幅。属性は立っていない
  await expect(page.locator('html')).not.toHaveAttribute('data-width', 'full');
  expect(await toggleLabel(page)).toBe('全幅にする');

  // 52rem。style.css の .wrap と揃える唯一の数字なので、rem から解く
  const rem = await page.evaluate(() =>
    parseFloat(getComputedStyle(document.documentElement).fontSize),
  );
  const { wrap, viewport } = await widths(page);
  expect(wrap).toBeCloseTo(52 * rem, 0);
  // 前提: viewport の方が広い。そうでなければ全幅にする意味自体が無い
  expect(viewport).toBeGreaterThan(wrap);

  // 本題: この幅では図が入り切らない
  expect(await figureOverflows(page)).toBe(true);

  await page.screenshot({ path: path.join(SHOTS, '04-width-narrow.png'), fullPage: true });
});

test('全幅にすると図が丸ごと入る', async ({ page }) => {
  await page.goto(PAGE_URL);
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('横に広い図');

  await page.locator('.widthtoggle').click();

  await expect(page.locator('html')).toHaveAttribute('data-width', 'full');
  // ラベルは「今押すとどうなるか」に入れ替わる
  expect(await toggleLabel(page)).toBe('標準幅にする');

  // 本文枠が画面いっぱいになる
  const { wrap, viewport } = await widths(page);
  expect(wrap).toBeCloseTo(viewport, 0);

  // 本題: 図が枠に収まる
  expect(await figureOverflows(page)).toBe(false);

  await page.screenshot({ path: path.join(SHOTS, '05-width-full.png'), fullPage: true });
});

test('全幅は再読み込みしても保ち、もう一度押すと戻る', async ({ page }) => {
  await page.goto(PAGE_URL);
  await page.locator('.widthtoggle').click();
  await expect(page.locator('html')).toHaveAttribute('data-width', 'full');

  // localStorage に載っているので、読み直しても標準幅へ落ちない。
  // <head> で同期に立てているため、描画前から full のまま
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-width', 'full');
  expect(await figureOverflows(page)).toBe(false);

  // 別ページへ移っても保つ (状態は chrome 側が持つ)
  await page.goto('/');
  await expect(page.locator('html')).toHaveAttribute('data-width', 'full');

  await page.goBack();
  await page.locator('.widthtoggle').click();
  await expect(page.locator('html')).not.toHaveAttribute('data-width', 'full');
  expect(await figureOverflows(page)).toBe(true);

  await page.reload();
  await expect(page.locator('html')).not.toHaveAttribute('data-width', 'full');
});
