(() => {
  if (window.top !== window || location.origin !== "https://ltxlib.dpdns.org")
    return;
  const model = globalThis.JavBossLtxCatalog;
  const panel = document.createElement("aside");
  panel.className = "javboss-ltx-sync";
  panel.setAttribute("aria-label", "JavBoss LTX 目录同步");
  const details = document.createElement("details");
  const heading = document.createElement("summary");
  heading.textContent = "JavBoss · LTX 目录同步";
  const status = document.createElement("p");
  status.setAttribute("role", "status");
  status.textContent =
    "同步完成后，JavBoss 可显示本站收录角标。只读取番号和状态，不下载视频。";
  const start = document.createElement("button");
  start.type = "button";
  start.textContent = "同步 / 继续同步";
  const reset = document.createElement("button");
  reset.type = "button";
  reset.textContent = "从头同步";
  const stop = document.createElement("button");
  stop.type = "button";
  stop.textContent = "暂停";
  stop.disabled = true;
  details.append(heading, status, start, reset, stop);
  panel.appendChild(details);
  document.body.appendChild(panel);
  let running = false;
  let stopped = false;
  const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
  const text = (el) => (el?.textContent || "").trim();
  const findButton = (value) =>
    Array.from(document.querySelectorAll("button")).find(
      (el) => !panel.contains(el) && text(el) === value,
    );
  const send = async (action, payload) => {
    const response = await chrome.runtime.sendMessage({
      type: "JAVBOSS_LTX_CATALOG",
      action,
      payload,
    });
    if (!response?.ok)
      throw new Error(response?.error || "扩展连接已中断，请刷新页面后重试");
    return response;
  };
  async function waitFor(read, timeout = 30000) {
    const deadline = Date.now() + timeout;
    while (Date.now() < deadline) {
      if (stopped) throw new Error("已暂停，检查点和旧缓存已保留");
      const value = read();
      if (value) return value;
      await pause(150);
    }
    throw new Error(
      "页面加载超时或需要验证，请在浏览器处理后继续；检查点已保留",
    );
  }
  function readPage() {
    const pager = document.querySelector("nav.main-pager");
    if (
      !pager ||
      pager.querySelector('input[aria-label="跳转到页码"]')?.disabled
    )
      return null;
    const match = text(pager.querySelector(".pager-summary")).match(
      /第\s*(\d+)\s*\/\s*(\d+)\s*页/,
    );
    const count = (title) =>
      Number(
        text(document.querySelector(`button[title="${title}"]`)).replace(
          /\D/g,
          "",
        ),
      );
    if (!match) return null;
    return {
      page: Number(match[1]),
      pages: Number(match[2]),
      total: count("全部作品"),
      confirmed: count("Jasna 汇总表已收录，信息与链接已核对"),
      pending: count("未收录于 Jasna 汇总表，信息或链接待确认"),
      items: Array.from(
        document.querySelectorAll("article.card[data-code]"),
      ).map((el) => ({
        code: el.getAttribute("data-code"),
        pending: Boolean(
          el.querySelector('.badge[title="未收录于 Jasna 汇总表，待确认"]'),
        ),
      })),
    };
  }
  async function goTo(page) {
    const current = await waitFor(readPage);
    if (current.page === page) return current;
    const input = document.querySelector(
      'nav.main-pager input[aria-label="跳转到页码"]',
    );
    const form = input?.closest("form");
    if (!input || !form) throw new Error("找不到分页控件，站点结构可能已变化");
    // Use the site's normal controls, never custom requests or encrypted API hooks.
    input.value = String(page);
    input.dispatchEvent(new Event("input", { bubbles: true }));
    form.requestSubmit();
    return waitFor(() => {
      const result = readPage();
      return result?.page === page ? result : null;
    });
  }
  async function browseMode(mode) {
    let control = findButton(mode);
    const settings = document.querySelector('button[aria-label="设置"]');
    if (!control) {
      settings?.click();
      control = await waitFor(() => findButton(mode));
    }
    const wasWaterfall =
      findButton("瀑布流")?.getAttribute("aria-pressed") === "true";
    try {
      if (
        mode === "分页" &&
        document
          .querySelector('button[title="列表中隐藏已添加与想看（想看页保留）"]')
          ?.getAttribute("aria-checked") === "true"
      )
        throw new Error("请先在新站设置中关闭“隐藏已下载”，再同步完整目录");
      if (control.getAttribute("aria-pressed") !== "true") control.click();
    } finally {
      if (settings?.getAttribute("aria-expanded") === "true") settings.click();
    }
    return wasWaterfall;
  }
  async function sync(restart) {
    if (running) return;
    running = true;
    stopped = false;
    start.disabled = reset.disabled = true;
    stop.disabled = false;
    let restoreWaterfall = false;
    try {
      if (
        location.search ||
        document.querySelector('input[aria-label="搜索作品"]')?.value ||
        document
          .querySelector('button[title="全部作品"]')
          ?.getAttribute("aria-pressed") !== "true"
      )
        throw new Error(
          "请切换为简体中文，打开 https://ltxlib.dpdns.org/ 首页并清空搜索和筛选，再同步完整目录",
        );
      if (document.querySelector('button[title="返回全部"]'))
        throw new Error("请退出“想看”或其他个人筛选，再同步完整目录");
      if (restart) await send("clear");
      const saved = (await send("load")).checkpoint;
      restoreWaterfall = await browseMode("分页");
      // Hidden/downloaded and want filters must not silently produce a partial index.
      const first = await goTo(1);
      model.validatePage(first, 1, first);
      const snapshot = {
        total: first.total,
        confirmed: first.confirmed,
        pending: first.pending,
        page: 0,
        pages: first.pages,
        items: [],
      };
      if (
        saved &&
        (saved.total !== first.total ||
          saved.confirmed !== first.confirmed ||
          saved.pending !== first.pending)
      )
        throw new Error("目录计数已变化，请使用“从头同步”；旧缓存保留");
      for (let page = 1; page <= first.pages; page++) {
        status.textContent = `${saved && page <= saved.page ? "核对检查点" : "读取目录"}：${page} / ${first.pages} 页，请勿更改筛选或关闭此标签页。`;
        const current = await goTo(page);
        model.validatePage(current, page, first);
        if (
          saved &&
          page <= saved.page &&
          JSON.stringify(current.items) !==
            JSON.stringify(saved.items.slice((page - 1) * 60, page * 60))
        )
          throw new Error("目录内容已变化，请使用“从头同步”；旧缓存保留");
        snapshot.items = model.append(snapshot.items, current);
        snapshot.page = page;
        if (!saved || page >= saved.page) await send("checkpoint", snapshot);
        if (page < first.pages) await pause(1000);
      }
      if (stopped) throw new Error("已暂停，检查点和旧缓存已保留");
      model.complete(snapshot);
      stop.disabled = true;
      status.textContent = "完整性核对通过，正在保存到 JavBoss…";
      const result = await send("commit", snapshot);
      status.textContent = `同步成功：${result.total} 条（已确认 ${snapshot.confirmed} / 待确认 ${snapshot.pending}）。JavBoss 约一分钟内刷新角标。`;
    } catch (error) {
      status.textContent = error?.message || "同步失败，旧缓存保留";
    } finally {
      // Best effort: a closed tab cannot restore its temporary browse preference.
      if (restoreWaterfall) {
        const wasStopped = stopped;
        stopped = false;
        try {
          await browseMode("瀑布流");
        } catch {
          /* Restore on the next manual visit if verification blocks UI. */
        }
        stopped = wasStopped;
      }
      running = false;
      start.disabled = reset.disabled = false;
      stop.disabled = true;
    }
  }
  start.addEventListener("click", () => sync(false));
  reset.addEventListener("click", () => sync(true));
  stop.addEventListener("click", () => {
    stopped = true;
  });
})();
