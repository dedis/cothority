"use strict";

const topl = require('topl');
const UUID = require('pure-uuid');
const protobuf = require('protobufjs')

const misc = require('../misc/misc.js');

const BASE64 = require("base-64");
const UTF8 = require("utf8");

/**
 * Socket is a WebSocket object instance through which protobuf messages are
 * sent to conodes.
 * @param {url} string conode identity
 * @param {path} string websocket path 
 * @param {object} protobufjs root messages. Usually just 
 *              use `require("cothority.protobuf").root`
 *
 * @throws {TypeError} when url is not a string or protobuf is not an object
 */
function Socket(node, path, messages) {
    if (typeof protobuf !== 'object')
	throw new TypeError;

    this.url = convertServerIdentityToWebSocket(node, path);
    this.protobuf = messages;

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

/**
 * Parse cothority roster toml string into a JavaScript object.
 * @example
 * // Toml needs to adhere to the following format
 * // where public has to be a base64 encodable string.
 *
 *     [[servers]]
 *         Address = "tcp://127.0.0.1:7002"
 *         Public = "GhxOf6H+23gK2NP4qu+FrRT5/Ca08+tCRcAaoZu26BY="
 *         Description = "Conode_1"
 *     [[servers]]
 *         Address = "tcp://127.0.0.1:7004"
 *         Public = "HSSppBPaE4QFpPQ2yvDN9Fss/RIe/jmtEvNvMm3y49M="
 *         Description = "Conode_2"
 *
 * Where public has to be a base64 encodable string.
 * @param {string} toml of the above format.
 *
 * @throws {TypeError} when toml is not a string
 * @return {object} roster
 */
function parseCothorityRoster(toml) {
    if (typeof toml !== 'string')
        throw new TypeError;

    const roster = topl.parse(toml);
    roster.servers.forEach((server) => {
        const pub = Uint8Array.from(ATOB(server.Public), c => c.charCodeAt(0));
        const url = 'https://dedis.epfl.ch/id/' + misc.uint8ArrayToHex(pub);
        server.Id = new UUID(5, 'ns:URL', url).export();
    });

    return roster;
}

/**
 * Gets a single conode from a roster toml string as a JavaScript object.
 * @param toml - the toml string
 * @param hexKey - the key string (hex)
 * @returns {object} - the conode object or undefined if not found
 */
function getConodeFromRoster(toml, hexKey) {
  if (typeof toml !== "string" || typeof hexKey !== "string") {
    throw new TypeError;
  }

  let roster = parseCothorityRoster(toml);
  let wantedConode = undefined;

  roster.servers.forEach((conode) => {
    if (conode.Public === hexKey) {
      wantedConode = conode;
    }
  });

  return wantedConode;
}

function ATOB(string) {
  return BASE64.encode(UTF8.encode(string));
}

/**
 * Convert a server identity url to a websocket url.
 *
 * @param {serverIdentity} object
 * @returns {string} websocket url
 */
function convertServerIdentityToWebSocket(serverIdentity, path){
    let parts = serverIdentity.Address.replace("tcp://", "").split(":");
    parts[1] = parseInt(parts[1]) + 1;

    return "ws://" + parts.join(":") + path;
}
