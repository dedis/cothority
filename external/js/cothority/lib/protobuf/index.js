"use strict";

const jsonDescriptor = require("./models.json");
const protobuf = require("protobufjs/light");
const root = protobuf.Root.fromJSON(jsonDescriptor);

function removeTypePrefix(buffer) {
  return buffer.slice(16, buffer.length);
}

module.exports.root = root;
module.exports.removeTypePrefix = removeTypePrefix;
