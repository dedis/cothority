const chai = require("chai");
const expect = chai.expect;

const cothority = require("../../lib");

const rStr = `
  [[servers]]
Address = "tls://localhost:7002"
Suite = "Ed25519"
Public = "c24a4e46b5d6e1d61c0b370019f98086c0828c30b56c2efa6f21c774e45049c2"
Description = "Conode_1"
  [[servers]]
Address = "tls://localhost:7004"
Suite = "Ed25519"
Public = "c351c08f62ba0dfd317ed439e81f2a72a88b91de3d1f6d4676e8cfaa99640dfa"
Description = "Conode_2"
  [[servers]]
Address = "tls://localhost:7006"
Suite = "Ed25519"
Public = "0d75f6903e7fbcb5e8623c942f707e4d36fbfbfdefdd7ae8b50633d0ed86a3a1"
Description = "Conode_3"
`;

describe("get config", () => {
  it("can get config", () => {
    const roster = cothority.Roster.fromTOML(rStr);
    const rs = new cothority.net.RosterSocket(roster, "ByzCoin");
    const id = cothority.misc.hexToUint8Array("ffc52147a1a33e15015d3cbf2a1df96dea4a4e3a4525f921fe74f7d2467bbd70");
    cothority.byzcoin.ByzCoinRPC.fromKnownConfiguration(rs, id).then(bc => {
      expect(bc.skipchainID).to.equal(id);
    });
  });
});