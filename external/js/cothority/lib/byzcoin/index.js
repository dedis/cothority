const ByzCoinRPC = require("./ByzCoinRPC");

const contracts = require("./contracts");
const darc = require("./darc");

module.exports = {
  contracts: contracts,
  darc: darc
};

module.exports.ByzCoinRPC = ByzCoinRPC;
