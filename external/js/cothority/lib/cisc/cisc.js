"use strict";

const net = require("../net");
const protobuf = require("../protobuf");
const misc = require("../misc");
const identity = require("../identity.js");

const kyber = require("@dedis/kyber-js");

const co = require("co");

class Client {
  /**
   * Returns a new cisc client from a roster
   *
   * @param {cothority.Roster} roster the roster over which the client can talk to
   * @param {string} lastID known skipblock/genesis ID in hexadecimal format
   * @returns {CiscClient} A client that can talks to the cisc services
   */
  constructor(roster, lastID) {
    this.lastRoster = roster;
    this.lastID = misc.hexToUint8Array(lastID);
  }

  /**
   * getLatestCISCData asks for the latest CISC block and returns the raw CISC data
   * @return {Promise} A promise which resolves with the latest cisc data if
   * all checks pass.
   */
  getLatestCISCData() {
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

  /**
   * getLatestCISCData asks for the latest CISC block and returns the data in "storage"
   * @return {Promise} A promise which resolves with the latest KV storage
   */
  getStorage() {
    var fn = co.wrap(function*(client){
      const ciscBlock = yield client.getLatestCISCData();
      const kvStore = ciscBlock.data.storage;
      return Promise.resolve(kvStore);
    });
    return fn(this);
  }

}

module.exports.Client = Client;
