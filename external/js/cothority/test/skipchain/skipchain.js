//"use strict";
//var wtf = require("wtfnode");
const chai = require("chai");
const expect = chai.expect;

const cothority = require("../../lib");
const proto = cothority.protobuf;
const skipchain = cothority.skipchain;
const misc = cothority.misc;
const net = cothority.net;
const kyber = require("@dedis/kyber-js");

const helpers = require("../helpers.js");

const curve = new kyber.curve.edwards25519.Curve();
const child_process = require("child_process");
const co = require("co");

describe.only("skipchain client", () => {
  after(function() {
    helpers.killGolang();
  });

  it("can retrieve updates from conodes", done => {
    const build_dir = process.cwd() + "/test/skipchain/build";
    var last_block;
    var roster, id;
    var fn = co.wrap(function*() {
      [roster, id] = helpers.readSkipchainInfo(build_dir);
      const client = new skipchain.Client(curve, roster, id);
      last_block = yield client.getLatestBlock();
      //console.log(last_block);
      // try to read it from a roster socket
      //  and compare if we have the same results
      const socket = new net.RosterSocket(roster, "Skipchain");
      const requestStr = "GetUpdateChain";
      const responseStr = "GetUpdateChainReply";
      const request = { latestId: misc.hexToUint8Array(id) };
      allBlocks = yield socket.send(requestStr, responseStr, request);
      var length = allBlocks.update.length;
      lastReceived = allBlocks.update[length - 1];
      expect(lastReceived).to.be.deep.equal(last_block);
      done();
    });
    helpers
      .runGolang(build_dir)
      .then(fn)
      .catch(err => {
        done();
        throw err;
      });
  });
});
