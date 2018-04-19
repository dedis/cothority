"use strict";

const topl = require("topl");
const UUID = require("pure-uuid");
const co = require("co");
const protobuf = require("protobufjs");
const kyber = require("@dedis/kyber-js");

const misc = require("./misc");

/**
 * ServerIdentity represents a cothority server. It has an associated public key
 * and a TCP address. The WebSocket address is derived from the TCP address.
 * */
class ServerIdentity {
  /*
     * Returns a new ServerIdentity from the public key and address.
     * Defaults to using Websocket Secure (wss) if the address has
     * tls as the protocol and Websocket (ws) otherwise
     * @param {Uint8Array} public key in bytes format
     * @param {string} address tcp address of the cothority node
     * @param {string} description of the conode. Can be null.
     * @param {boolean} wss to connect using WebSocket Secure (port 443)
     * @return a ServerIdentity
     * */
  constructor(group, publicKey, address, description, wss) {
    if (!(publicKey instanceof kyber.Point)) throw new TypeError();
    if (!(group instanceof kyber.Group)) throw new TypeError();
    this.group = group;
    this.pub = publicKey;
    this.addr = address;
    this._description = description;
    // id of the identity
    const hex = misc.uint8ArrayToHex(this.pub.marshalBinary());
    const url = "https://dedis.epfl.ch/id/" + hex;
    this._id = new UUID(5, "ns:URL", url).export();
    // tcp + websocket address
    let parts = address.split("://");
    if (parts.length != 2) {
      throw new Error("invalid address: " + address);
    }
    // XXX Does not support IPv6 yet
    let fullAddress = parts[1].split(":");
    fullAddress[1] = parseInt(fullAddress[1]) + 1;
    if (wss) {
      this.wsAddr = `wss://${fullAddress[0]}`;
    } else {
      this.wsAddr = `ws://${fullAddress[0]}:${fullAddress[1]}`;
    }
  }

  /**
   * Returns a new ServerIdentity from the public key in hexadecimal format
   * and address
   * @param {string} hexPublic public key
   * @param {string} address
   * @param {string} description
   * @param {boolean} wss to connect using WebSocket Secure (port 443)
   * @return a ServerIdentity
   * */
  static fromHexPublic(group, hexPublic, address, description, wss) {
    var pubBuff = misc.hexToUint8Array(hexPublic);
    var pub = group.point();
    pub.unmarshalBinary(pubBuff);
    return new ServerIdentity(group, pub, address, description, wss);
  }
  /*
     * @return the public key as a Uint8Array buffer
     * */
  get public() {
    return this.pub;
  }

  /*
     * @return the WebSocket address. That can be passed into the net.Socket
     * address constructor argument.
     * */
  get websocketAddr() {
    return this.wsAddr;
  }

  /*
     * @return the tcp address of the server
     * */
  get tcpAddr() {
    return this.addr;
  }

  /*
     * @return the id of this serveridentity
     * */
  get id() {
    return this._id;
  }

  /**
   * returns the description associated with this identity
   *
   * @returns {string} the string description
   */
  get description() {
    return this._description;
  }

  toString() {
    return this.tcpAddr;
  }
}

/*
 * Roster represents a group of servers. It basically consists in a list of
 * ServerIdentity with some helper functions.
 * */
class Roster {
  /*
     * @param a list of ServerIdentity
     * @return a Roster from the given list of identites
     * */
  constructor(group, identities, id) {
    this.group = group;
    this._identities = identities;
    this._id = id;
  }

  /*
     * Random selects a random identity and returns it
     * @return a random identity
     * */
  random() {
    const idx = Math.floor(Math.random() * (this.length() - 1));
    return this._identities[idx];
  }

  /*
     * @return the list of identities composing this Roster
     * */
  get identities() {
    return this._identities;
  }

  get(idx) {
    if (idx >= this.identitis.length) throw new Error("identity idx too high");

    return this.identities[idx];
  }

  /*
     * @return the length of the roster
     * */
  get length() {
    return this._identities.length;
  }

  /*
     * @return the id of the roster
     * */
  get id() {
    return this._id;
  }

  /**
   * aggregateKey returns the aggregate public key for this server.
   * It is the sum of all public keys of the identities of this Roster.
   *
   * @param {kyber.Group} group The group to use to compute the aggregate key.
   * @returns {kyber.Point} The aggregate key
   */
  aggregateKey() {
    const aggr = this.group.point().null();
    for (var i = 0; i < this.length; i++) {
      aggr.add(aggr, this.identities[i].public);
    }
    return aggr;
  }

  /**
   * Parse cothority roster toml string into a Roster object.
   * @example
   * // Toml needs to adhere to the following format
   * // where public has to be a hex-encoded string.
   *
   *    [[servers]]
   *        Address = "tcp://127.0.0.1:7001"
   *        Public = "4e3008c1a2b6e022fb60b76b834f174911653e9c9b4156cc8845bfb334075655"
   *        Description = "conode1"
   *    [[servers]]
   *        Address = "tcp://127.0.0.1:7003"
   *        Public = "e5e23e58539a09d3211d8fa0fb3475d48655e0c06d83e93c8e6e7d16aa87c106"
   *        Description = "conode2"
   *
   * @param {kyber.Group} group to construct the identities
   * @param {string} toml of the above format.
   * @param {boolean} wss to connect using WebSocket Secure (port 443)
   *
   * @throws {TypeError} when toml is not a string
   * @return {Roster} roster
   */
  static fromTOML(toml, wss) {
    if (typeof toml !== "string") throw new TypeError();

    const roster = topl.parse(toml);
    var group = roster.Suite === undefined ? "edwards25519" : roster.Suite;
    group = kyber.curve.newCurve(group);
    const identities = roster.servers.map(server =>
      ServerIdentity.fromHexPublic(
        group,
        server.Public,
        server.Address,
        server.description,
        wss
      )
    );
    return new Roster(group, identities);
  }

  /**
   * Parses the protobuf-decoded object into a Roster object
   *
   * @static
   * @param {Object} protoRoster the litteral JS object returned by protobuf
   * @param {boolean} wss to connect using WebSocket Secure (port 443)
   * @returns {Roster} the Roster object
   */
  static fromProtobuf(protoRoster, wss) {
    var group =
      protoRoster.Suite === undefined ? "edwards25519" : protoRoster.Suite;
    group = kyber.curve.newCurve(group);
    const identities = protoRoster.list.map(id => {
      var pub = group.point();
      pub.unmarshalBinary(new Uint8Array(id.public));
      return new ServerIdentity(group, pub, id.address, id.description, wss);
    });
    return new Roster(group, identities);
  }
}

module.exports = {
  Roster,
  ServerIdentity
};
