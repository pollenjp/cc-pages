import { test, expect, type Page } from '@playwright/test';
import * as path from 'node:path';

// cc-pages が <pre class="diff"> を行ごとに着色することの e2e。
//
// 撮っていないもの (撮れないものではなく、意図的に範囲外):
// - セッション一覧 / ページ一覧。diff の着色と関係しない
// - standalone モードのページ。アプリの CSS が当たらないので着色も掛からない
// - ライト/ダーク以外の配色。prefers-color-scheme はこの 2 値しか持たない

const PAGE_URL = '/p/20260101-e2efixture/' + encodeURIComponent('0001-diff-の配色') + '/';
const SHOTS = path.join(__dirname, 'screenshots');

// CSS カスタムプロパティを、ブラウザに解決させて rgb(...) で受け取る。
//
// 期待値に #e7f7ec のような 16 進を書き写すと、style.css と e2e の 2 箇所に
// 同じ色が住むことになり、片方だけ直したときに気付けない。トークンを引いて
// 比べる形なら、色を変えても「トークンが効いている」ことの検査であり続ける。
async function tokenColor(page: Page, name: string): Promise<string> {
  return page.evaluate((n) => {
    const probe = document.createElement('span');
    probe.style.color = `var(${n})`;
    document.body.appendChild(probe);
    const resolved = getComputedStyle(probe).color;
    probe.remove();
    return resolved;
  }, name);
}

/** 行の class ごとに、背景がトークンの色と一致することを見る。 */
async function expectDiffColors(page: Page) {
  await expect(page.locator('pre.diff .a').first()).toHaveCSS(
    'background-color',
    await tokenColor(page, '--diff-add-bg'),
  );
  await expect(page.locator('pre.diff .d').first()).toHaveCSS(
    'background-color',
    await tokenColor(page, '--diff-del-bg'),
  );
  await expect(page.locator('pre.diff .h').first()).toHaveCSS(
    'color',
    await tokenColor(page, '--accent'),
  );
  await expect(page.locator('pre.diff .m').first()).toHaveCSS(
    'color',
    await tokenColor(page, '--muted'),
  );
}

test.describe('light', () => {
  test.use({ colorScheme: 'light' });

  test('diff の配色 (light)', async ({ page }) => {
    await page.goto(PAGE_URL);
    // 真っ白でも通る「撮るだけの test」にしない。このページ固有の見出しを見る
    await expect(page.getByRole('heading', { level: 1 })).toHaveText('diff の配色');

    await expectDiffColors(page);
    await page.screenshot({ path: path.join(SHOTS, '01-diff-light.png'), fullPage: true });
  });
});

test.describe('dark', () => {
  test.use({ colorScheme: 'dark' });

  test('diff の配色 (dark)', async ({ page }) => {
    await page.goto(PAGE_URL);
    await expect(page.getByRole('heading', { level: 1 })).toHaveText('diff の配色');

    await expectDiffColors(page);
    await page.screenshot({ path: path.join(SHOTS, '02-diff-dark.png'), fullPage: true });
  });
});

test('横スクロールしても背景が行の端まで続く', async ({ page }) => {
  await page.goto(PAGE_URL);
  const wide = page.locator('#wide');
  await expect(wide).toBeVisible();

  // 右端まで送ってから撮る。ここで背景が切れていれば絵で分かる
  await wide.evaluate((pre) => {
    pre.scrollLeft = pre.scrollWidth;
  });
  await wide.screenshot({ path: path.join(SHOTS, '03-diff-scrolled.png') });

  const geom = await wide.evaluate((pre) => {
    const code = pre.querySelector('code') as HTMLElement;
    const rows = Array.from(pre.querySelectorAll('span')) as HTMLElement[];
    return {
      codeWidth: code.getBoundingClientRect().width,
      preClientWidth: pre.clientWidth,
      rowWidths: rows.map((r) => r.getBoundingClientRect().width),
    };
  });

  // 前提: この pre は実際に溢れている。溢れていなければ以下の検査は無意味
  expect(geom.codeWidth).toBeGreaterThan(geom.preClientWidth);

  // 本題: すべての行が最長行と同じ幅を持つ。どれか 1 行でも短いと、
  // 右へスクロールしたときにその行だけ背景が途切れる
  for (const w of geom.rowWidths) {
    expect(Math.abs(w - geom.codeWidth)).toBeLessThan(1.5);
  }
});

test('class="diff" の無い pre は素通しされる', async ({ page }) => {
  await page.goto(PAGE_URL);
  await expect(page.locator('#plain')).toBeVisible();
  await expect(page.locator('#plain span')).toHaveCount(0);
});
