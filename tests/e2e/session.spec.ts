import { test, expect, type Page } from '@playwright/test';
import * as path from 'node:path';

// セッションの状況板 (/p/<session>/ の「現在の状況」「次やること」) の e2e。
//
// 撮っていないもの (撮れないものではなく、意図的に範囲外):
// - status 無しのセッション。fixture は 1 セッションしか持たず、空状態の文言は
//   Go 側の TestSessionPageWithoutStatusShowsPlaceholder が字面で見ている
// - / の一覧。状況は出さない設計で、Go 側の TestListPageDoesNotShowStatus が見ている

const SESSION_URL = '/p/20260101-e2efixture/';
const SHOTS = path.join(__dirname, 'screenshots');

const status = (page: Page) => page.locator('section.status');

async function openSession(page: Page) {
  await page.goto(SESSION_URL);
  // 真っ白でも通る「撮るだけの test」にしない。状況板そのものを見る
  await expect(status(page)).toBeVisible();
}

/** 状況板の中身が fixture の status.json と一致することを見る。 */
async function expectStatus(page: Page) {
  await expect(status(page).getByRole('heading', { name: '現在の状況' })).toBeVisible();
  await expect(status(page).getByRole('heading', { name: '次やること' })).toBeVisible();
  // now は ul、next は ol (順序に意味がある)
  await expect(status(page).locator('ul li')).toHaveText([
    'diff の着色・本文幅トグル・画像の拡大を e2e で検査済み',
    'セッションの状況板を実装中',
  ]);
  await expect(status(page).locator('ol li')).toHaveText([
    'status.json の fixture でスクショを撮る',
    'PR の sticky コメントで見た目を確認する',
  ]);

  // 状況板はページ一覧より上にある
  const statusTop = await status(page).evaluate((el) => el.getBoundingClientRect().top);
  const listTop = await page
    .getByRole('heading', { name: 'ページ', exact: true })
    .evaluate((el) => el.getBoundingClientRect().top);
  expect(statusTop).toBeLessThan(listTop);
}

test.describe('light', () => {
  test.use({ colorScheme: 'light' });

  test('セッションの状況板 (light)', async ({ page }) => {
    await openSession(page);
    await expectStatus(page);
    await page.screenshot({ path: path.join(SHOTS, '08-session-status-light.png'), fullPage: true });
  });
});

test.describe('dark', () => {
  test.use({ colorScheme: 'dark' });

  test('セッションの状況板 (dark)', async ({ page }) => {
    await openSession(page);
    await expectStatus(page);
    await page.screenshot({ path: path.join(SHOTS, '09-session-status-dark.png'), fullPage: true });
  });
});
