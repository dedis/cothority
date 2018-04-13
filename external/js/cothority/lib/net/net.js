"use strict";

const topl = require("topl");
const UUID = require("pure-uuid");
const protobuf = require("protobufjs");
const co = require("co");
const shuffle = require("crypto-shuffle");
const WS = require("ws");

const root = require("../protobuf/index.js").root;
const identity = require("../identity.js");

/**
 * Socket is a WebSocket object instance through which protobuf messages are
 * sent to conodes.
 * @param {string} addr websocket address of the conode to contact.
 * @param {string} service name. A socket is tied to a service name.
 *
 * @throws {TypeError} when url is not a string or protobuf is not an object
 */
function Socket(addr, service) {
  if (typeof protobuf !== "object") throw new TypeError();

  this.url = addr + "/" + service;
  this.protobuf = root;

  /**
   * Send transmits data to a given url and parses the response.
   * @param {string} request name of registered protobuf message
   * @param {string} response name of registered protobuf message
   * @param {object} data to be sent
   *
   * @returns {object} Promise with response message on success, and an error on failure
   */
  this.send = (request, response, data) => {
    return new Promise((resolve, reject) => {
      const path = this.url + "/" + request;
      console.log("net.Socket: new WebSocket(" + path + ")");
      const ws = new WS(this.url + "/" + request);
      ws.binaryType = "arraybuffer";

      const requestModel = this.protobuf.lookup(request);
      if (requestModel === undefined)
        reject(new Error("Model " + request + " not found"));

      const responseModel = this.protobuf.lookup(response);
      if (responseModel === undefined)
        reject(new Error("Model " + response + " not found"));

      // This makes the API consistent with nativescript-websockets
      if (typeof ws.open === "function") {
        ws._notify = ws._notifyBrowser;
        Object.defineProperty(WS.prototype, "_notify", {
          enumerable: false
        });

        Object.defineProperty(ws, "_notify", { enumerable: false });

        ws.open();
      }

      ws.onopen = () => {
        const errMsg = requestModel.verify(data);
        if (errMsg) {
          reject(new Error(errMsg));
        }
        const message = requestModel.create(data);
        const marshal = requestModel.encode(message).finish();
        ws.send(marshal);
      };

      ws.onmessage = event => {
        ws.close();
        const { data } = event;
        let buffer;
        if (ws.android) {
          data.rewind();
          const len = data.limit();
          buffer = new Uint8Array(len);
          for (let i = 0; i < len; i++) {
            buffer[i] = data.get(i);
          }
        } else {
          buffer = new Uint8Array(data);
        }
        const unmarshal = responseModel.decode(buffer);
        resolve(unmarshal);
      };

      ws.onclose = event => {
        if (!event.wasClean || event.code === 4000) {
          reject(new Error(event.reason));
        }
      };

      ws.onerror = error => {
        reject(error);
      };
    });
  };
}

/*
 * RosterSocket offers similar functionality from the Socket class but chooses
 * a random conode when trying to connect. If a connection fails, it
 * automatically retries to connect to another random server.
 * */
class RosterSocket {
  constructor(roster, service) {
    this.addresses = roster.identities.map(id => id.websocketAddr);
    this.service = service;
    this.lastGoodServer = null;
  }

  /**
   * send tries to send the request to a random server in the list as long as there is no successful response. It tries a permutation of all server's addresses.
   *
   * @param {string} request name of the protobuf packet
   * @param {string} response name of the protobuf packet response
   * @param {Object} data javascript object representing the request
   * @returns {Promise} holds the returned data in case of success.
   */
  send(request, response, data) {
    const that = this;
    const fn = co.wrap(function*() {
      const addresses = that.addresses;
      const service = that.service;
      shuffle(addresses);
      // try first the last good server we know
      if (that.lastGoodServer) addresses.unshift(that.lastGoodServer);

      for (let i = 0; i < addresses.length; i++) {
        const addr = addresses[i];
        try {
          const socket = new Socket(addr, service);
          console.log("RosterSocket: trying out " + addr + "/" + service);
          const socketResponse = yield socket.send(request, response, data);
          that.lastGoodServer = addr;
          return Promise.resolve(socketResponse);
        } catch (err) {
          console.error("rostersocket: " + err);
          continue;
        }
      }
      return Promise.reject(new Error("no conodes are available"));
    });
    return fn();
  }
}

/**
 * LeaderSocket reads a roster and can be used to communicate with the leader
 * node. As of now the leader is the first node in the roster.
 *
 * @throws {Error} when roster doesn't have any node
 */
class LeaderSocket {
  constructor(roster, service) {
    this.service = service;
    this.roster = roster;

    if (this.roster.identities.length === 0) {
      throw new Error("Roster should have atleast one node");
    }
  }

  /**
   * Send transmits data to a given url and parses the response.
   * @param {string} request name of registered protobuf message
   * @param {string} response name of registered protobuf message
   * @param {object} data to be sent
   *
   * @returns {Promise} with response message on success and error on failure.
   */
  send(request, response, data) {
    // fn is a generator that tries the sending the request to the leader
    // maximum 3 times and returns on the first successful attempt
    const that = this;
    const fn = co.wrap(function*() {
      for (let i = 0; i < 3; i++) {
        try {
          const socket = new Socket(
            that.roster.identities[0].websocketAddr,
            that.service
          );
          const reply = yield socket.send(request, response, data);
          return Promise.resolve(reply);
        } catch (e) {
          console.error("error sending request: ", e.message);
        }
      }
      return Promise.reject(
        new Error("couldn't send request after 3 attempts")
      );
    });
    return fn();
  }
}

module.exports = {
  Socket,
  RosterSocket,
  LeaderSocket
};
