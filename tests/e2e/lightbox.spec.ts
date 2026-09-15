import { test, expect, type Page } from '@playwright/test';
import * as path from 'node:path';

// 画像の拡大 (overlay) と「別タブで画像のみを開く」の e2e。
//
// 撮っていないもの (撮れないものではなく、意図的に範囲外):
// - standalone のページ。chrome ごと被らないので lightbox.js も載らない。
//   Go 側の TestStandalonePageHasNoChrome が字面で見ている
// - ダークモード。overlay の面は var(--bg) = 本文と同じ配色なので、
//   配色そのものは diff.spec.ts が撮っている分で足りる
// - 一覧 / セッションのページ。<article> が無く、対象の画像も無い

const PAGE_URL = '/p/20260101-e2efixture/' + encodeURIComponent('0003-画像の拡大') + '/';
const SHOTS = path.join(__dirname, 'screenshots');

/** fixture の原寸。assets/shot.svg と index.html の inline SVG に書いてある値。 */
const SHOT_NATURAL_WIDTH = 1600;
const FLOW_NATURAL_WIDTH = 1400;

const overlay = (page: Page) => page.locator('dialog.lightbox');
const stage = (page: Page) => page.locator('dialog.lightbox .lightbox-stage');
const media = (page: Page) => page.locator('dialog.lightbox .lightbox-media');
const openInTab = (page: Page) => overlay(page).getByRole('link', { name: '別タブで開く' });

/** 本文の画像 2 枚。alt で引く。 */
const shot = (page: Page) => page.getByAltText('設定画面のスクリーンショット');
const flow = (page: Page) => page.locator('article svg#flow-figure');

async function openPage(page: Page) {
  await page.goto(PAGE_URL);
  // 真っ白でも通る「撮るだけの test」にしない。このページ固有の見出しを見る
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('画像の拡大');
}

/** overlay の中で画像が枠に収まらず、スクロールが要る状態か。 */
async function stageScrolls(page: Page): Promise<boolean> {
  return stage(page).evaluate((el) => el.scrollWidth - el.clientWidth > 1);
}

/** overlay に出ている画像の、実際に描かれている幅。 */
async function mediaWidth(page: Page): Promise<number> {
  return media(page).evaluate((el) => el.getBoundingClientRect().width);
}

async function scrollPos(page: Page): Promise<{ left: number; top: number }> {
  return stage(page).evaluate((el) => ({ left: el.scrollLeft, top: el.scrollTop }));
}

test('画像をクリックすると overlay が開き、Esc で閉じる', async ({ page }) => {
  await openPage(page);
  // 開くまでは overlay の要素自体が無い (JS が初回に組み立てる)
  await expect(overlay(page)).toHaveCount(0);

  await shot(page).click();
  await expect(overlay(page)).toBeVisible();
  await expect(media(page)).toBeVisible();

  await page.keyboard.press('Escape');
  await expect(overlay(page)).toBeHidden();
});

test('overlay の外側をクリックすると閉じる', async ({ page }) => {
  await openPage(page);
  await shot(page).click();
  await expect(overlay(page)).toBeVisible();

  // 画像の外 = 左上の隅。ここは stage の余白で、画像には当たらない
  await stage(page).click({ position: { x: 4, y: 4 } });
  await expect(overlay(page)).toBeHidden();
});

test('閉じるボタンで閉じる', async ({ page }) => {
  await openPage(page);
  await shot(page).click();
  await overlay(page).getByRole('button', { name: '閉じる' }).click();
  await expect(overlay(page)).toBeHidden();
});

test('overlay は最初フィット、画像をクリックすると原寸になる', async ({ page }) => {
  await openPage(page);
  await shot(page).click();

  // フィット: 原寸より縮んでいて、スクロールは要らない
  await expect(overlay(page)).toHaveAttribute('data-zoom', 'fit');
  expect(await mediaWidth(page)).toBeLessThan(SHOT_NATURAL_WIDTH);
  expect(await stageScrolls(page)).toBe(false);
  await page.screenshot({ path: path.join(SHOTS, '06-lightbox-fit.png') });

  // 原寸: 自然な大きさそのままで、はみ出す分はスクロールで見る
  await media(page).click();
  await expect(overlay(page)).toHaveAttribute('data-zoom', 'actual');
  expect(await mediaWidth(page)).toBeCloseTo(SHOT_NATURAL_WIDTH, 0);
  expect(await stageScrolls(page)).toBe(true);
  await page.screenshot({ path: path.join(SHOTS, '07-lightbox-actual.png') });

  // もう一度押すとフィットへ戻る
  await media(page).click();
  await expect(overlay(page)).toHaveAttribute('data-zoom', 'fit');
});

test('原寸ではドラッグで表示位置を動かせる', async ({ page }) => {
  await openPage(page);
  await shot(page).click();
  await media(page).click();
  await expect(overlay(page)).toHaveAttribute('data-zoom', 'actual');

  const before = await scrollPos(page);
  await page.mouse.move(600, 500);
  await page.mouse.down();
  await page.mouse.move(400, 400, { steps: 10 });
  await page.mouse.up();

  // 左上へ引いた分だけ、右下が見えるようにスクロールする
  const after = await scrollPos(page);
  expect(after.left).toBeGreaterThan(before.left);
  expect(after.top).toBeGreaterThan(before.top);

  // ドラッグの終わりを「原寸を解除するクリック」と取り違えない
  await expect(overlay(page)).toHaveAttribute('data-zoom', 'actual');
});

test('assets の画像は別タブで元の URL がそのまま開く', async ({ page, context }) => {
  await openPage(page);
  await shot(page).click();

  await expect(openInTab(page)).toHaveAttribute('href', /\/assets\/shot\.svg$/);
  const [opened] = await Promise.all([context.waitForEvent('page'), openInTab(page).click()]);
  await opened.waitForLoadState();

  expect(opened.url()).toContain('/assets/shot.svg');
  // 画像だけの文書。chrome は被っていない
  await expect(opened.locator('header.bar')).toHaveCount(0);
});

test('inline SVG の図もクリックで拡大できる', async ({ page }) => {
  await openPage(page);
  await flow(page).click();

  await expect(overlay(page)).toBeVisible();
  expect(await mediaWidth(page)).toBeLessThan(FLOW_NATURAL_WIDTH);
  await media(page).click();
  expect(await mediaWidth(page)).toBeCloseTo(FLOW_NATURAL_WIDTH, 0);

  // overlay に出るのは複製なので、同じ id が 2 つになっていないこと
  await expect(page.locator('#flow-figure')).toHaveCount(1);
});

test('inline SVG は別タブで blob の画像として開く', async ({ page, context }) => {
  await openPage(page);
  await flow(page).click();

  await expect(openInTab(page)).toHaveAttribute('href', /^blob:/);
  const [opened] = await Promise.all([context.waitForEvent('page'), openInTab(page).click()]);
  await opened.waitForLoadState();

  expect(opened.url()).toMatch(/^blob:/);
  // 画像そのものが開いている = 文書のルートが <svg>
  expect(await opened.evaluate(() => document.documentElement.tagName)).toBe('svg');
});

test('既に <a> に包まれている画像は overlay を開かず、リンク先へ飛ぶ', async ({ page }) => {
  await openPage(page);
  // overlay が割り込むとここで止まる (遷移が起きない)
  await page.getByAltText('元の画像へのリンク').click();
  await page.waitForURL(/\/assets\/shot\.svg$/);
});

test('本文の画像を Ctrl+クリックすると overlay を挟まず別タブで開く', async ({ page, context }) => {
  await openPage(page);
  const [opened] = await Promise.all([
    context.waitForEvent('page'),
    shot(page).click({ modifiers: ['ControlOrMeta'] }),
  ]);
  await opened.waitForLoadState();

  expect(opened.url()).toContain('/assets/shot.svg');
  await expect(overlay(page)).toBeHidden();
});
