const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const model = require("../content/ltx-catalog-model.js");
const source = fs.readFileSync(
  path.join(__dirname, "../content/ltx-catalog.js"),
  "utf8",
);

// A local DOM/control fixture: no website requests, browser installation or real
// library writes. The same isolated script drives normal pagination controls.
function harness({
  saved = null,
  failCommit = false,
  pauseAt = 0,
  search = "",
  collision = false,
} = {}) {
  let current = 1,
    opened = false,
    mode = "瀑布流",
    checkpoint = saved,
    commits = [],
    visited = [],
    panel;
  const nodes = [];
  const node = (tag = "button", label = "") => ({
    tagName: tag.toUpperCase(),
    textContent: label,
    children: [],
    listeners: {},
    disabled: false,
    attrs: {},
    value: "",
    setAttribute(k, v) {
      this.attrs[k] = v;
    },
    getAttribute(k) {
      return this.attrs[k] ?? null;
    },
    append(...children) {
      this.children.push(...children);
    },
    appendChild(child) {
      this.children.push(child);
    },
    contains(child) {
      return this.children.some((c) => c === child || c.contains(child));
    },
    addEventListener(type, cb) {
      this.listeners[type] = cb;
    },
    dispatchEvent() {},
    click() {
      return this.listeners.click?.();
    },
    closest() {
      return form;
    },
  });
  const settings = node("button", "设置");
  settings.getAttribute = (k) =>
    k === "aria-expanded" ? String(opened) : null;
  settings.click = () => {
    opened = !opened;
  };
  const waterfall = node("button", "瀑布流"),
    pagination = node("button", "分页");
  for (const control of [waterfall, pagination]) {
    control.getAttribute = (k) =>
      k === "aria-pressed" ? String(mode === control.textContent) : null;
    control.click = () => {
      mode = control.textContent;
    };
  }
  const input = node("input");
  const form = {
    requestSubmit() {
      current = Number(input.value);
      visited.push(current);
    },
  };
  const countNode = (text) => ({
    textContent: text,
    getAttribute: (k) => (k === "aria-pressed" ? "true" : null),
  });
  const document = {
    body: {
      appendChild(el) {
        panel = el;
      },
    },
    createElement(tag) {
      const el = node(tag);
      nodes.push(el);
      return el;
    },
    querySelector(selector) {
      if (selector === 'input[aria-label="搜索作品"]') return { value: "" };
      if (selector === 'button[aria-label="设置"]') return settings;
      if (selector === 'button[title="全部作品"]') return countNode("全部65");
      if (selector === 'button[title="Jasna 汇总表已收录，信息与链接已核对"]')
        return countNode("已确认64");
      if (
        selector === 'button[title="未收录于 Jasna 汇总表，信息或链接待确认"]'
      )
        return countNode("待确认1");
      if (selector === 'nav.main-pager input[aria-label="跳转到页码"]')
        return input;
      if (selector === "nav.main-pager")
        return mode === "分页"
          ? {
              querySelector(s) {
                if (s.startsWith("input")) return input;
                if (s === ".pager-summary")
                  return { textContent: `第 ${current} / 2 页` };
                return null;
              },
            }
          : null;
      return null;
    },
    querySelectorAll(selector) {
      if (selector === "button")
        return [
          settings,
          ...(opened ? [waterfall, pagination] : []),
          ...nodes.filter((el) => el.tagName === "BUTTON"),
        ];
      if (selector === "article.card[data-code]")
        return Array.from({ length: current === 1 ? 60 : 5 }, (_, i) => ({
          getAttribute() {
            return `ABC-${collision && current === 2 && i === 1 ? 0 : (current - 1) * 60 + i}`;
          },
          querySelector() {
            return current === 2 && i === 0 ? {} : null;
          },
        }));
      return [];
    },
  };
  const actions = [];
  const chrome = {
    runtime: {
      async sendMessage(message) {
        actions.push(message.action);
        if (message.action === "load")
          return {
            ok: true,
            checkpoint: checkpoint ? structuredClone(checkpoint) : null,
          };
        if (message.action === "clear") {
          checkpoint = null;
          return { ok: true };
        }
        if (message.action === "checkpoint") {
          checkpoint = structuredClone(message.payload);
          if (pauseAt === checkpoint.page)
            nodes.find((el) => el.textContent === "暂停").click();
          return { ok: true };
        }
        if (message.action === "commit") {
          if (failCommit)
            return { ok: false, error: "test connection failure" };
          commits.push(structuredClone(message.payload));
          checkpoint = null;
          return { ok: true, total: message.payload.total };
        }
        throw Error(message.action);
      },
    },
  };
  const window = {};
  window.top = window;
  vm.runInNewContext(source, {
    window,
    location: { origin: "https://ltxlib.dpdns.org", search },
    document,
    chrome,
    JavBossLtxCatalog: model,
    Event: class {},
    setTimeout: (callback) => setImmediate(callback),
    Date,
  });
  return {
    run: (restart = false) =>
      nodes
        .find(
          (el) => el.textContent === (restart ? "从头同步" : "同步 / 继续同步"),
        )
        .click(),
    get checkpoint() {
      return checkpoint;
    },
    commits,
    actions,
    visited,
    status: () => nodes.find((el) => el.tagName === "P").textContent,
    mode: () => mode,
    panel: () => panel,
  };
}

test("isolated sync script reads all pages, commits exact counts and restores waterfall mode", async () => {
  const h = harness();
  await h.run();
  assert.equal(h.commits.length, 1);
  assert.equal(h.commits[0].items.length, 65);
  assert.equal(h.commits[0].pending, 1);
  assert.equal(h.mode(), "瀑布流");
  assert.match(h.status(), /同步成功/);
  assert.equal(h.checkpoint, null);
});
test("pause retains progress, and resume revalidates previously read pages before committing", async () => {
  const first = harness({ pauseAt: 1 });
  await first.run();
  assert.equal(first.commits.length, 0);
  assert.equal(first.checkpoint.page, 1);
  assert.match(first.status(), /暂停/);
  const resumed = harness({ saved: first.checkpoint });
  await resumed.run();
  assert.equal(resumed.commits.length, 1);
  assert.equal(resumed.commits[0].items.length, 65);
  const changed = structuredClone(first.checkpoint);
  changed.items[0].code = "CHANGED-001";
  const rejected = harness({ saved: changed });
  await rejected.run();
  assert.equal(rejected.commits.length, 0);
  assert.match(rejected.status(), /目录内容已变化/);
});
test("failures, duplicate codes and filtered pages never commit a partial catalogue", async () => {
  const failed = harness({ failCommit: true });
  await failed.run();
  assert.equal(failed.commits.length, 0);
  assert.equal(failed.checkpoint.items.length, 65);
  assert.match(failed.status(), /connection failure/);
  const duplicate = harness({ collision: true });
  await duplicate.run();
  assert.equal(duplicate.commits.length, 0);
  assert.match(duplicate.status(), /重复番号/);
  const filtered = harness({ search: "?actor=test" });
  await filtered.run();
  assert.equal(filtered.actions.length, 0);
  assert.match(filtered.status(), /清空/);
});
