"use strict";

const topl = require('topl');
const UUID = require('pure-uuid');
const protobuf = require('protobufjs')

const root = require('../protobuf/index.js').root;

const BASE64 = require("base-64");
const UTF8 = require("utf8");

/**
 * Socket is a WebSocket object instance through which protobuf messages are
 * sent to conodes.
 * @param {path} string websocket path. Composed from a node's address with the
 *              websocket's service name
 * @param {object} protobufjs root messages. Usually just
 *              use `require("cothority.protobuf").root`
 *
 * @throws {TypeError} when url is not a string or protobuf is not an object
 */
function Socket(path) {
    if (typeof protobuf !== 'object')
	    throw new TypeError;

    this.url = path;
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
           const ws = new WebSocket(this.url + '/' + request);
           ws.binaryType = 'arraybuffer';

           const requestModel = this.protobuf.lookup(request);
           if (requestModel === undefined)
               reject(new Error('Model ' + request + ' not found'));

           const responseModel = this.protobuf.lookup(response);
           if (responseModel === undefined)
               reject(new Error('Model ' + response + ' not found'));

           ws.onopen = () => {
               const message = requestModel.create(data);
               const marshal = requestModel.encode(message).finish();
               ws.send(marshal);
           };

           ws.onmessage = (event) => {
               ws.close();
               const buffer = new Uint8Array(event.data);
               const unmarshal = responseModel.decode(buffer);
               resolve(unmarshal);
           };

           ws.onclose = (event) => {
               if (!event.wasClean)
                   reject(new Error(event.reason));
           };

           ws.onerror = (error) => {
               reject(new Error('Could not connect to ' + this.url));
           };
	});
   };
}

module.exports.Socket = Socket;
