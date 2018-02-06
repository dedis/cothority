"use strict";

const chai = require("chai");
const expect = chai.expect;

const cothority = require("../lib");

const serversTOML = `
[[servers]]
  Address = "tcp://127.0.0.1:7001"
  Public = "4e3008c1a2b6e022fb60b76b834f174911653e9c9b4156cc8845bfb334075655"
  Description = "conode1"

[[servers]]
  Address = "tcp://127.0.0.1:7003"
  Public = "e5e23e58539a09d3211d8fa0fb3475d48655e0c06d83e93c8e6e7d16aa87c106"
  Description = "conode2"
`
describe("roster", () => {
    
    it("can be created", () => {
        const roster = new cothority.Roster([1,2,3]);
        expect(roster).to.not.be.null;
        expect(roster instanceof cothority.Roster).to.be.true;
        expect(roster.constructor === cothority.Roster).to.be.true;
    });

    it("is correctly parsed", () => {
        const roster = cothority.Roster.fromTOML(serversTOML);
        expect(roster.length).to.be.equals(2);
        expect(roster.identities[0].tcpAddr).to.be.equal("tcp://127.0.0.1:7001");
    }); 

    it("gives correct websocket address", () => {
        const roster = cothority.Roster.fromTOML(serversTOML);
        const wsAddr = roster.identities[0].websocketAddr;
        expect(wsAddr).to.be.equal("ws://127.0.0.1:7002");
    });

});
