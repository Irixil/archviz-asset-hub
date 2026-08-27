const path = require("node:path");

function resolveAssetPath(root, storageKey) {
  if (!root || !storageKey || path.isAbsolute(storageKey)) {
    throw new Error("invalid asset path");
  }

  const rootPath = path.resolve(root);
  const filePath = path.resolve(rootPath, storageKey.split("/").join(path.sep));
  const relative = path.relative(rootPath, filePath);
  if (
    !relative ||
    relative.startsWith(`..${path.sep}`) ||
    path.isAbsolute(relative)
  ) {
    throw new Error("asset path escapes storage root");
  }
  return filePath;
}

module.exports = { resolveAssetPath };
