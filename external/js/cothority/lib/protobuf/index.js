"use strict";

const jsonDescriptor = require("./models.json");
const protobuf = require("protobufjs/light");
const root = protobuf.Root.fromJSON(jsonDescriptor);

module.exports.root = root;
