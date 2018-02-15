"use strict";

const topl = require("topl");
const UUID = require("pure-uuid");
const protobuf = require("protobufjs");
const co = require("co");
const shuffle = require("crypto-shuffle");
const WebSocket = require("isomorphic-ws");

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
      const ws = new WebSocket(this.url + "/" + request);
      ws.binaryType = "arraybuffer";

      const requestModel = this.protobuf.lookup(request);
      if (requestModel === undefined)
        reject(new Error("Model " + request + " not found"));

      const responseModel = this.protobuf.lookup(response);
      if (responseModel === undefined)
        reject(new Error("Model " + response + " not found"));

      ws.onopen = () => {
        const message = requestModel.create(data);
        const marshal = requestModel.encode(message).finish();
        ws.send(marshal);
      };

      ws.onmessage = event => {
        ws.close();
        const buffer = new Uint8Array(event.data);
        const unmarshal = responseModel.decode(buffer);
        resolve(unmarshal);
      };

      ws.onclose = event => {
        if (!event.wasClean) reject(new Error(event.reason));
      };

      ws.onerror = error => {
        reject(new Error("could not connect to " + this.url + ":" + error));
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
    const socket = this;
    const fn = co.wrap(function*(req, resp, data, socket) {
      request = req;
      response = resp;
      data = data;
      var addresses = socket.addresses;
      var service = socket.service;
      shuffle(addresses);
      // try first the last good server we know
      if (socket.lastGoodServer) addresses.unshift(socket.lastGoodServer);

      for (var i = 0; i < addresses.length; i++) {
        var addr = addresses[i];
        try {
          var socket = new Socket(addr, service);
          console.log("RosterSocket: trying out " + addr + "/" + service);
          var data = yield socket.send(request, response, data);
          socket.lastGoodServer = addr;
          return Promise.resolve(data);
        } catch (err) {
          console.log("rostersocket: " + err);
          continue;
        }
      }
      return Promise.reject("no conodes are available");
    });
    return fn(request, response, data, socket);
  }
}

module.exports.Socket = Socket;
module.exports.RosterSocket = RosterSocket;
