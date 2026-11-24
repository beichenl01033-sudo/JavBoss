/* Shared pure validation for the isolated LTX content script and its tests. */
(() => {
  const key = (value) =>
    String(value || "")
      .trim()
      .toUpperCase()
      .replace(/[-_ ]/g, "");
  function validatePage(page, expected, initial) {
    if (
      !Number.isSafeInteger(page.total) ||
      page.total <= 0 ||
      page.total > 20000 ||
      !Number.isSafeInteger(page.confirmed) ||
      !Number.isSafeInteger(page.pending) ||
      page.confirmed < 0 ||
      page.pending < 0 ||
      page.pages !== Math.ceil(page.total / 60) ||
      page.confirmed + page.pending !== page.total ||
      page.page !== expected ||
      page.total !== initial.total ||
      page.confirmed !== initial.confirmed ||
      page.pending !== initial.pending ||
      page.pages !== initial.pages
    )
      throw new Error("目录数量或筛选条件已变化，请从头同步；旧缓存保留");
    const expectedCount =
      expected === page.pages ? page.total - (expected - 1) * 60 : 60;
    if (
      page.items.length !== expectedCount ||
      !page.items.every(
        (item) =>
          /^[A-Za-z0-9][A-Za-z0-9 ._-]{0,127}$/.test(item.code) &&
          typeof item.pending === "boolean",
      )
    )
      throw new Error("页面未完整加载或站点结构已变化，旧缓存保留");
  }
  function append(items, page) {
    const seen = new Set(items.map((item) => key(item.code)));
    for (const item of page.items) {
      if (!key(item.code) || seen.has(key(item.code)))
        throw new Error("目录出现重复番号，请从头同步；旧缓存保留");
      seen.add(key(item.code));
    }
    return [...items, ...page.items];
  }
  function complete(snapshot) {
    if (
      snapshot.items.length !== snapshot.total ||
      new Set(snapshot.items.map((item) => key(item.code))).size !==
        snapshot.total ||
      snapshot.items.filter((item) => item.pending).length !==
        snapshot.pending ||
      snapshot.confirmed + snapshot.pending !== snapshot.total
    )
      throw new Error("目录尚未完整同步，旧缓存保留");
    return snapshot;
  }
  const model = { key, validatePage, append, complete };
  if (typeof module !== "undefined" && module.exports) module.exports = model;
  else globalThis.JavBossLtxCatalog = model;
})();
