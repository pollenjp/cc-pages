import { test, expect, type Page } from '@playwright/test';
import * as path from 'node:path';

// 会話のトップページの「リンク」の節 (cc-pages link が links.json に残したもの) と、
// session id から会話のトップページへ飛ぶ /s/<session_id>/ の e2e。
//
// 撮っていないもの (撮れないものではなく、意図的に範囲外):
// - リンクの無い会話。fixture は 1 会話しか持たず、節を出さないことは Go 側の
//   TestSessionPageHidesLinksWhenNone が字面で見ている
// - javascript: などを <a> にしないこと。links.json に危ない URL を置くとスクショに
//   出てしまうので、Go 側の TestSessionPageDoesNotLinkUnsafeURL に任せる

const SESSION_URL = '/p/20260101-e2efixture/';
const SHOTS = path.join(__dirname, 'screenshots');

const links = (page: Page) => page.locator('section.links');

async function openSession(page: Page) {
  await page.goto(SESSION_URL);
  // 真っ白でも通る「撮るだけの test」にしない。リンクの節そのものを見る
  await expect(links(page)).toBeVisible();
}

/** リンクの節の中身が fixture の links.json と合うことを見る。 */
async function expectLinks(page: Page) {
  await expect(links(page).getByRole('heading', { level: 2 })).toHaveText('リンク (6)');
  // 組は、組の中で一番早い最初の時刻の順。label の無い件は「その他」
  await expect(links(page).locator(':scope > h3')).toHaveText(['チケット', 'PR', 'cc-page', 'その他']);
  // 組の中は最後の時刻の新しい順 (repo#12 は翌日まで出ていた)
  await expect(links(page).locator(':scope > h3 + ul').nth(1).getByRole('link')).toHaveText([
    'repo#12',
    'repo#11',
  ]);
  // cc-pages 自身のページは、添え書きにそのページの題
  await expect(links(page).locator('li', { hasText: '📄0001' }).locator('.desc')).toHaveText('diff の配色');
  // ほかの添え書きは scheme を除いた URL
  await expect(links(page).locator('li', { hasText: 'TKT-12' }).locator('.desc')).toHaveText(
    'app.notion.com/p/e2e-ticket-12',
  );

  // リンクの節はページの一覧より下にある
  const listTop = await page
    .getByRole('heading', { name: 'ページ', exact: true })
    .evaluate((el) => el.getBoundingClientRect().top);
  const linksTop = await links(page).evaluate((el) => el.getBoundingClientRect().top);
  expect(linksTop).toBeGreaterThan(listTop);
}

/** tool の出力だけに出たリンクは畳んであり、開くと見える。 */
async function expectToolLinksFolded(page: Page) {
  const details = links(page).locator('details');
  await expect(details).not.toHaveAttribute('open', '');
  await expect(details.locator('summary')).toHaveText('tool の出力だけに出たリンク (1)');
  const run = links(page).getByRole('link', { name: 'CI run' });
  await expect(run).toBeHidden();
  await details.locator('summary').click();
  await expect(run).toBeVisible();
}

for (const scheme of ['light', 'dark'] as const) {
  test.describe(scheme, () => {
    test.use({ colorScheme: scheme });

    test(`会話のトップページのリンクの節 (${scheme})`, async ({ page }) => {
      await openSession(page);
      await expectLinks(page);
      await expectToolLinksFolded(page);
      const n = scheme === 'light' ? '10' : '11';
      await page.screenshot({ path: path.join(SHOTS, `${n}-session-links-${scheme}.png`), fullPage: true });
    });
  });
}

test('/s/<session_id>/ は会話のトップページへ送る', async ({ page }) => {
  await page.goto('/s/e2e-fixture/');
  await expect(page).toHaveURL(new RegExp(`${SESSION_URL}$`));
  await expect(links(page)).toBeVisible();
});

test('/s/ で会話が見つからなければ 404 のページ', async ({ page }) => {
  const res = await page.goto('/s/no-such-session/');
  expect(res?.status()).toBe(404);
  await expect(page.getByText('この会話のページ・リンクはまだありません')).toBeVisible();
  await expect(page.getByRole('link', { name: '一覧へ' })).toHaveAttribute('href', '/');
});

test('一覧の会話の行にリンクの数が出る', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('section.session').first().locator(':scope > .meta')).toContainText('6 リンク');
});
