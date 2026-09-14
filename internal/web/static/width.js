// 本文幅のトグル。標準 (52rem センター寄せ) ⇄ 全幅 (画面いっぱい) を
// <html data-width="full"> の有無で切り替える。状態は localStorage に持つので
// 全ページ共通、リロードしても戻らない。
//
// <head> から同期で読むこと。属性を最初の描画より前に立てないと、52rem で一瞬
// 出てから広がる (逆も) ちらつきが出る。defer / async にすると必ず起きる。
//
// インラインに書かないのは CSP を緩めないため。script-src は未指定で
// default-src 'self' に落ちるので、同一オリジンのこのファイルはそのまま通る。
// フラグメントのインライン <script> が禁止のままなのは変わらない。
(function () {
  var KEY = "cc-pages:width";
  var root = document.documentElement;

  function apply(v) {
    if (v === "full") {
      root.setAttribute("data-width", "full");
    } else {
      root.removeAttribute("data-width");
    }
  }

  // localStorage は private mode や site data のブロックで読み書き自体が例外を
  // 投げる。幅を思い出せなくても表示は壊れないので、握り潰して既定 (標準幅) に倒す。
  try {
    apply(localStorage.getItem(KEY));
  } catch (e) {
    /* 既定 = 標準幅 */
  }

  // クリックは document への委譲で拾う。head の時点でボタンはまだ存在しないが
  // document は既にあるので、DOMContentLoaded を待たずにここで登録できる。
  document.addEventListener("click", function (ev) {
    var t = ev.target;
    if (!t || !t.closest || !t.closest(".widthtoggle")) return;
    var next = root.getAttribute("data-width") === "full" ? "narrow" : "full";
    apply(next);
    try {
      localStorage.setItem(KEY, next);
    } catch (e) {
      /* 保持できないだけ。今の表示は既に切り替わっている */
    }
  });
})();
