"use strict";

const BN = require("bn.js");

/**
 * @module constants
 */

/**
 * Constants
 * @typedef {Object} constants
 * @property {BN.jsObject} zeroBN - BN.js object representing 0
 */
const constants = {
  zeroBN: new BN(0)
};

module.exports = constants;
