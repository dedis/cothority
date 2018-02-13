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

describe.only("skipchain client", () => {
  after(function() {
    helpers.killGolang();
  });

  it("can retrieve updates from conodes", done => {
    const build_dir = process.cwd() + "/test/skipchain/build";
    helpers
      .runGolang(build_dir)
      .then(data => {
        [roster, id] = helpers.readSkipchainInfo(build_dir);
        const client = new skipchain.Client(curve, roster, id);
        return client.getLatestBlock();
      })
      .then(data => {
        console.log(data);
        expect(true).to.be.true;
        done();
      })
      .catch(err => {
        throw err;
      });
  });
});
