const protobuf = require("protobufjs");
const fs = require("fs");
const files = require("file");

const root = new protobuf.Root();
root.define("cothority");

const regex = /^.*\.proto$/;

files.walk("lib/protobuf/build/models", (err, path, dirs, items) => {
  items.forEach(file => {
    console.log(file);
    if (regex.test(file)) {
      root.loadSync(file);
    }
  });

  fs.writeFileSync("lib/protobuf/models.json", JSON.stringify(root.toJSON()));
});
