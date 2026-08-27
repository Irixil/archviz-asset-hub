const assert = require("node:assert/strict");
const path = require("node:path");
const test = require("node:test");
const { resolveAssetPath } = require("./path.cjs");

test("maps a storage key beneath the configured shared root", () => {
  assert.equal(
    resolveAssetPath("/shared/storage", "workspace/asset/model.max"),
    path.resolve("/shared/storage/workspace/asset/model.max"),
  );
});

test("rejects absolute and traversal paths", () => {
  assert.throws(() => resolveAssetPath("/shared/storage", "/etc/passwd"));
  assert.throws(() => resolveAssetPath("/shared/storage", "../secret.max"));
});
