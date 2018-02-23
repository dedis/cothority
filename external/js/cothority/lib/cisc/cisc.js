"use strict";

const net = require("../net");
const protobuf = require("../protobuf");
const misc = require("../misc");
const identity = require("../identity.js");
const skipchain = require("../skipchain");

const kyber = require("@dedis/kyber-js");

const co = require("co");

class Client {
  /**
   * Returns a new cisc client from a roster
   *
   * @param {kyber.Group} the group to use for crypto operations
   * @param {cothority.Roster} roster the roster over which the client can talk to
   * @param {string} lastID known skipblock/genesis ID in hexadecimal format
   * @returns {CiscClient} A client that can talks to the cisc services
   */
  constructor(group, roster, lastID) {
    this.lastRoster = roster;
    this.lastID = misc.hexToUint8Array(lastID);
    this.group = group;
    this.skipchain = new skipchain.Client(group, roster, lastID);
  }

  /**
   * getLatestCISCDataUnsafe asks for the latest CISC block and returns the raw CISC data
   * @return {Promise} A promise which resolves with the latest cisc data if
   * all checks pass.
   */
  getLatestCISCDataUnsafe() {
    var fn = co.wrap(function*(client) {
      const requestStr = "DataUpdate";
      const responseStr = "DataUpdateReply";
      const request = { id: client.lastID };

      // fetches the data with the current roster
      client.socket = new net.RosterSocket(client.lastRoster, "Identity");

      var data = null;
      try {
        data = yield client.socket.send(requestStr, responseStr, request);
        return Promise.resolve(data);
      } catch (err) {
        return Promise.reject(err);
      }
    });
    return fn(this);
  }

  getLatestCISCData() {
    var fn = co.wrap(function*(client) {
      const block = yield client.skipchain.getLatestBlock();
      // interpret block as cisc block
      const data = block.data;
      const dataModel = protobuf.root.lookup("Data");
      const buffer = new Uint8Array(data);
      const prunedBuffer = protobuf.removeTypePrefix(buffer);
      const ciscData = dataModel.decode(prunedBuffer);
      console.log(ciscData.storage);
      return ciscData;
    });
    return fn(this);
  }

  /**
   * getLatestCISCData asks for the latest CISC block and returns the data in "storage"
   * @return {Promise} A promise which resolves with the latest KV storage
   */
  getStorage() {
    var fn = co.wrap(function*(client) {
      const ciscBlock = yield client.getLatestCISCData();
      console.log("cisc data retrieved and verified");
      const kvStore = ciscBlock.storage;
      return Promise.resolve(kvStore);
    });
    return fn(this);
  }
  /**
   * getLatestCISCData asks for the latest CISC block and returns the data in "storage"
   * @return {Promise} A promise which resolves with the latest KV storage
   */
  getStorageUnsafe() {
    var fn = co.wrap(function*(client) {
      const ciscBlock = yield client.getLatestCISCDataUnsafe();
      const kvStore = ciscBlock.data.storage;
      return Promise.resolve(kvStore);
    });
    return fn(this);
  }
}

module.exports.Client = Client;
