const chai = require("chai");
const expect = chai.expect;
const co = require("co");

const cothority = require("../../lib");
const helpers = require("../helpers");

describe("get config", () => {
  // Clean up the byzcoin server started by build/main.go.
  var proc;
  after(function() {
    helpers.killGolang(proc);
  });

  it("can get config", done => {
    const build_dir = process.cwd() + "/test/byzcoin/build";

    const fn = co.wrap(function*() {
      [roster, id] = helpers.readSkipchainInfo(build_dir);
      const id2 = cothority.misc.hexToUint8Array(id);
      const rs = new cothority.net.RosterSocket(roster, "ByzCoin");
      cothority.byzcoin.ByzCoinRPC.fromKnownConfiguration(rs, id2).then(bc => {
        expect(bc.skipchainID).to.equal(id2);
        done();
      });
    });

    helpers
      .runGolang(build_dir, data => data.match(/OK/))
      .then(p => {
        proc = p;
        return Promise.resolve(true);
      })
      .then(fn)
      .catch(err => {
        done();
        throw err;
      });
  }).timeout(5000);
});
