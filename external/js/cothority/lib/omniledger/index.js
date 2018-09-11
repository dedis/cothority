const OmniledgerRPC = require("./OmniledgerRPC");

const contracts = require("./contracts");
const darc = require("./darc");

module.exports = {
  contracts: contracts,
  darc: darc
};

module.exports.OmniledgerRPC = OmniledgerRPC;
