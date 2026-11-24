const test = require("node:test");
const assert = require("node:assert/strict");
const {
  validatePage,
  append,
  complete,
} = require("../content/ltx-catalog-model.js");

function page(number, total = 65) {
  return {
    page: number,
    pages: 2,
    total,
    confirmed: total - 1,
    pending: 1,
    items: Array.from({ length: number === 1 ? 60 : total - 60 }, (_, i) => ({
      code: `ABC-${(number - 1) * 60 + i}`,
      pending: number === 2 && i === 0,
    })),
  };
}
test("full catalogue requires ordered pages, exact totals, unique normalized codes and matching statuses", () => {
  const first = page(1),
    last = page(2);
  validatePage(first, 1, first);
  validatePage(last, 2, first);
  const snapshot = { ...first, items: append(append([], first), last) };
  assert.equal(complete(snapshot).items.length, 65);
  assert.throws(() =>
    complete({ ...snapshot, items: snapshot.items.slice(1) }),
  );
  assert.throws(() => complete({ ...snapshot, pending: 2, confirmed: 63 }));
  assert.throws(() => validatePage({ ...last, total: 66 }, 2, first));
  assert.throws(() => validatePage({ ...last, page: 1 }, 2, first));
  assert.throws(() =>
    validatePage({ ...last, items: last.items.slice(1) }, 2, first),
  );
  assert.throws(() => append([{ code: "abc_60", pending: false }], last));
  assert.throws(() =>
    validatePage(
      { ...first, items: [{ code: "../bad", pending: false }] },
      1,
      first,
    ),
  );
});
