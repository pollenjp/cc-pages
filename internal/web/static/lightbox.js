// 本文の画像の拡大 (overlay) と、画像だけを別タブで開く導線。
//
// 対象は <article> の中の <img> と、入れ子でない <svg>。ただし <a> に包まれて
// いるものは触らない — 書き手が自分でリンクにした意図の方が優先する。この 2 つの
// 条件は style.css の cursor 指定と対になっているので、片方だけ変えないこと。
//
// クリックは width.js と同じく document への委譲で拾う。あちらと違って <head> で
// 同期に読む必要は無い (最初の描画より前に立てる属性が無く、ちらつかない) ので
// defer で読み込む。
//
// インラインに書かないのは CSP を緩めないため。script-src は未指定で
// default-src 'self' に落ちるので、同一オリジンのこのファイルはそのまま通る。
// フラグメントのインライン <script> が禁止のままなのは変わらない。
(function () {
  var FIT = "fit";
  var ACTUAL = "actual";

  var dlg = null; // overlay。最初にクリックされたときに 1 つだけ組み立てる
  var stage = null; // 画像を載せる、スクロールする枠
  var link = null; // 「別タブで開く」
  var source = null; // いま overlay に出している、本文側の元の要素

  // inline <svg> を別タブで開くための blob URL。1 要素につき 1 回だけ作る。
  //
  // revoke しないのは、開いたタブがその URL を参照し続けるため。overlay を
  // 閉じるたびに捨てると「別タブで開いた画像が後から真っ白になる」。図 1 枚
  // あたり数 KB で、ページを離れれば道連れに消える。
  var blobs = new WeakMap();

  function isSVG(el) {
    return el.tagName.toLowerCase() === "svg";
  }

  // hit はクリックされた節点から、拡大の対象になる画像を探す。無ければ null。
  function hit(node) {
    if (!node || !node.closest) return null;
    var el = node.closest("article img, article svg");
    if (!el) return null;
    // 書き手が既にリンクにしている画像はその意図を優先する
    if (el.closest("a")) return null;
    // overlay に出している複製自身 (overlay は <article> の中に置く)
    if (el.closest(".lightbox")) return null;
    // <svg> の中の <svg> は外側だけを対象にする
    if (isSVG(el) && el.parentElement && el.parentElement.closest("svg")) return null;
    return el;
  }

  // fileURL はその画像を単体で開くための URL。
  //
  // <img> は assets/ が既に単体で配信されているのでその src をそのまま使う。
  // inline <svg> はファイルの実体が無いので、その場で組み立てて blob にする。
  // 本文の CSS (currentColor や var(--fg)) は付いて行かないが、色を計算して
  // 焼き込むとダークモードのときに「白地に薄い色」になって読めなくなる。
  // 未指定のまま = 黒、ブラウザが単体の SVG に敷く白地の上で読めるので触らない。
  function fileURL(el) {
    if (!isSVG(el)) return el.currentSrc || el.src || "";
    var url = blobs.get(el);
    if (!url) {
      var xml = new XMLSerializer().serializeToString(el);
      url = URL.createObjectURL(new Blob([xml], { type: "image/svg+xml;charset=utf-8" }));
      blobs.set(el, url);
    }
    return url;
  }

  // naturalSize は「原寸」の寸法。<img> は元画像の画素数、<svg> は viewBox の
  // 座標系を原寸と見なす。どちらも取れないときは今描かれている大きさに倒す。
  function naturalSize(el) {
    if (!isSVG(el)) return { w: el.naturalWidth, h: el.naturalHeight };
    var vb = el.viewBox && el.viewBox.baseVal;
    if (vb && vb.width > 0 && vb.height > 0) return { w: vb.width, h: vb.height };
    var r = el.getBoundingClientRect();
    return { w: r.width, h: r.height };
  }

  // view は overlay に載せる要素を作る。
  //
  // <img> は src と alt だけを写した新しい要素にする。本文側の width 属性や
  // style="max-width:100%" を連れて来ると、overlay の中での大きさをこちらで
  // 決められなくなるため。<svg> は複製するしかないが、root の id だけは外す
  // (同じ id の要素が 2 つある状態を作らない)。中の id は url(#…) の参照が
  // 生きているので残す — 複製と元は同じ内容なので、どちらを指しても同じに描ける。
  function view(el) {
    var node;
    if (isSVG(el)) {
      node = el.cloneNode(true);
      node.removeAttribute("id");
    } else {
      node = document.createElement("img");
      node.src = el.currentSrc || el.src;
      node.alt = el.alt || "";
    }
    node.classList.add("lightbox-media");
    return node;
  }

  function media() {
    return stage.firstElementChild;
  }

  // setZoom は overlay の中の画像の大きさを決める。
  //
  // 大きさは複製の style 属性へ直接書く。フラグメントが元の要素に書いた
  // style="max-width:100%" が複製にも付いて来ており、stylesheet の規則では
  // 勝てないため (同じ style 属性を上書きすれば確実に勝てる)。
  function setZoom(mode) {
    var el = media();
    if (!el) return;
    if (mode === ACTUAL) {
      var size = naturalSize(source);
      el.style.maxWidth = "none";
      el.style.maxHeight = "none";
      el.style.width = size.w + "px";
      el.style.height = size.h + "px";
    } else {
      el.style.maxWidth = "100%";
      el.style.maxHeight = "100%";
      el.style.width = "auto";
      el.style.height = "auto";
    }
    dlg.setAttribute("data-zoom", mode);
  }

  // toggleZoom はフィットと原寸を往復する。
  //
  // 原寸にするとき、カーソルの下にあった点をそのまま同じ位置に残す。単に先頭へ
  // 寄せると、拡大した瞬間に見ていた場所を見失う。
  function toggleZoom(ev) {
    if (dlg.getAttribute("data-zoom") === ACTUAL) {
      setZoom(FIT);
      return;
    }
    var size = naturalSize(source);
    if (!(size.w > 0)) return; // まだ読み込めていない画像。フィットのままにする
    var before = media().getBoundingClientRect();
    var fx = before.width ? (ev.clientX - before.left) / before.width : 0.5;
    var fy = before.height ? (ev.clientY - before.top) / before.height : 0.5;
    setZoom(ACTUAL);
    var box = stage.getBoundingClientRect();
    stage.scrollLeft = fx * stage.scrollWidth - (ev.clientX - box.left);
    stage.scrollTop = fy * stage.scrollHeight - (ev.clientY - box.top);
  }

  // 原寸のときのドラッグ。pointer capture は使わない — capture すると後続の
  // click が捕まえた要素 (stage) に付け替えられ、「画像を押した」のか
  // 「外側を押した」のかを click 側で見分けられなくなる。
  var drag = null;
  var dragged = false;

  function onMove(ev) {
    if (!drag) return;
    var dx = ev.clientX - drag.x;
    var dy = ev.clientY - drag.y;
    if (Math.abs(dx) > 3 || Math.abs(dy) > 3) dragged = true;
    stage.scrollLeft = drag.left - dx;
    stage.scrollTop = drag.top - dy;
  }

  function onUp() {
    drag = null;
    window.removeEventListener("pointermove", onMove);
    window.removeEventListener("pointerup", onUp);
  }

  function build(parent) {
    dlg = document.createElement("dialog");
    dlg.className = "lightbox";
    dlg.setAttribute("aria-label", "画像の拡大");
    dlg.setAttribute("data-zoom", FIT);

    var bar = document.createElement("div");
    bar.className = "lightbox-bar";

    link = document.createElement("a");
    link.className = "lightbox-open";
    link.target = "_blank";
    link.rel = "noopener";
    link.textContent = "別タブで開く";

    var close = document.createElement("button");
    close.type = "button";
    close.className = "lightbox-close";
    close.textContent = "閉じる";
    close.addEventListener("click", function () {
      dlg.close();
    });

    bar.appendChild(link);
    bar.appendChild(close);

    stage = document.createElement("div");
    stage.className = "lightbox-stage";
    // showModal() は中の最初の focusable へ焦点を移す。放っておくと「別タブで
    // 開く」にフォーカスリングが出た状態で開くので、autofocus でこちらに取る。
    // tabindex="-1" の要素にプログラムから当てた焦点にはリングが出ない。
    // ついでに、原寸のときに矢印キーでスクロールできるようになる。
    stage.tabIndex = -1;
    stage.setAttribute("autofocus", "");

    stage.addEventListener("click", function (ev) {
      // ドラッグの終わりは「クリック」として数えない。次の pointerdown が
      // 下ろすので、ここで戻さない (戻すと、ドラッグの手を枠の外で離した回の
      // 取りこぼしが次のクリック 1 回を飲み込む)。
      if (dragged) return;
      if (ev.target === stage) {
        dlg.close();
        return;
      }
      toggleZoom(ev);
    });

    stage.addEventListener("pointerdown", function (ev) {
      dragged = false;
      if (ev.button !== 0) return;
      if (dlg.getAttribute("data-zoom") !== ACTUAL) return;
      if (ev.target === stage) return;
      drag = { x: ev.clientX, y: ev.clientY, left: stage.scrollLeft, top: stage.scrollTop };
      ev.preventDefault(); // 画像そのものが drag & drop で持ち上がらないように
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onUp);
    });

    // 閉じたら複製を捨てる。Esc (dialog が自前で閉じる) でもここを通る。
    dlg.addEventListener("close", function () {
      stage.textContent = "";
      dlg.setAttribute("data-zoom", FIT);
      source = null;
    });

    dlg.appendChild(bar);
    dlg.appendChild(stage);
    // <article> の中に置くのは、フラグメントが書いた `article svg rect {…}` の
    // ような CSS を複製にも効かせるため。showModal() で開いた dialog は top
    // layer に出るので、DOM 上どこに居ても見え方は変わらない。
    parent.appendChild(dlg);
  }

  function open(el) {
    if (!dlg) build(el.closest("article") || document.body);
    source = el;
    stage.textContent = "";
    stage.appendChild(view(el));
    var url = fileURL(el);
    link.href = url;
    link.hidden = !url;
    setZoom(FIT);
    dlg.showModal();
    stage.scrollTop = 0;
    stage.scrollLeft = 0;
  }

  function openTab(el) {
    var url = fileURL(el);
    if (url) window.open(url, "_blank", "noopener");
  }

  document.addEventListener("click", function (ev) {
    var el = hit(ev.target);
    if (!el) return;
    ev.preventDefault();
    // Ctrl / ⌘ を押していれば overlay を挟まず、その画像だけを別タブへ。
    // リンクに対する普段の操作と揃える (画像は <a> ではないので、こちらで拾う)。
    if (ev.ctrlKey || ev.metaKey) {
      openTab(el);
      return;
    }
    open(el);
  });

  // 中クリック。リンクと同じ「別タブで開く」に割り当て、Chrome の
  // autoscroll (押した場所に丸いカーソルが出るあれ) は出さない。
  document.addEventListener("auxclick", function (ev) {
    if (ev.button !== 1) return;
    var el = hit(ev.target);
    if (!el) return;
    ev.preventDefault();
    openTab(el);
  });
})();
