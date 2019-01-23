const protobuf = require("protobufjs");
const fs = require("fs");
const files = require("file");

const root = new protobuf.Root();
root.define("cothority");

const regex = /^.*\.proto$/;
const protoPath = "../../proto/";

files.walkSync(protoPath, (path, dirs, items) => {
  items.forEach(file => {
    const fullPath = path + "/" + file;
    console.log(fullPath);
    if (regex.test(fullPath)) {
      root.loadSync(fullPath);
    }
  });
});

const modelPath = "./src/protobuf/models.json";
fs.writeFileSync(modelPath, JSON.stringify(root.toJSON()));
console.log();
console.log("JSON models written in " + modelPath);
