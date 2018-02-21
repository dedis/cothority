//"use strict";
const chai = require("chai");
const cothority = require("../../lib");
const kyber = require("@dedis/kyber-js");
const helpers = require("../helpers.js");
const child_process = require("child_process");
const fs = require("fs");

const curve = new kyber.curve.edwards25519.Curve();
const proto = cothority.protobuf;
const skipchain = cothority.skipchain;
const misc = cothority.misc;
const net = cothority.net;
const expect = chai.expect;

const build_dir = process.cwd() + "/test/cisc/build";
describe.only("cisc client", () => {
  it("can retrieve updates from conodes", done => {
    var proc;
    after(function() {
      helpers.killGolang(proc);
    });
    helpers
      .runGolang(build_dir, data => data.match(/OK/))
      .then(proces => {
        proc = proces;
        [roster, id] = helpers.readSkipchainInfo(build_dir);

        const addr1 = roster.identities[0].websocketAddr;
        const socket = new net.Socket(addr1, "Identity");
        const requestStr = "DataUpdate";
        const responseStr = "DataUpdateReply";

        const request = { id: misc.hexToUint8Array(id) };

        //console.log("Sending data", request);
        return socket.send(requestStr, responseStr, request);
      })
      .then(data => {
        console.log("Received data from the identity skipchain:", data);
        console.log("Storage: ", data.data.storage);
        console.log("Keys: ", Object.keys(data.data.storage));
        done();
      });
  }).timeout(5000);
});