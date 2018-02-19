"use strict";

const chai = require("chai");
const expect = chai.expect;

const cothority = require("../lib");
const proto = require("../lib/protobuf").root;
const kyber = require("@dedis/kyber-js");

const curve = new kyber.curve.edwards25519.Curve();

const serversTOML = `
[[servers]]
  Address = "tcp://127.0.0.1:7001"
  Public = "4e3008c1a2b6e022fb60b76b834f174911653e9c9b4156cc8845bfb334075655"
  Description = "conode1"

[[servers]]
  Address = "tcp://127.0.0.1:7003"
  Public = "e5e23e58539a09d3211d8fa0fb3475d48655e0c06d83e93c8e6e7d16aa87c106"
  Description = "conode2"
`;
describe("roster", () => {
  console.log(kyber.curve.availableCurves());
  it("can be created", () => {
    const roster = new cothority.Roster([1, 2, 3]);
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

  it("correctly parses protobuf-decoded object", () => {
    const addr1 = "tcp://127.0.0.1:7000";
    const addr2 = "tcp://127.0.0.1:7000";
    const objectId1 = fakeIdentity(addr1);
    const objectId2 = fakeIdentity(addr2);
    const objectRoster = {
      list: [objectId1, objectId2],
      aggregate: new Uint8Array([7, 8])
    };
    const rosterProto = proto.lookup("Roster");
    const message = rosterProto.create(objectRoster);
    const marshalled = rosterProto.encode(message).finish();
    const rosterProto2 = proto.lookup("Roster");
    const unmarshalled = rosterProto2.decode(marshalled);
    const roster = cothority.Roster.fromProtobuf(unmarshalled);
    expect(roster.length).to.be.equal(2);
    expect(roster.identities[0].tcpAddr).to.be.equal(addr1);
  });

  it("correctly computes the aggregate key", () => {
    const pub1 = curve.point().pick();
    const pub2 = curve.point().pick();
    const id1 = new cothority.ServerIdentity(
      curve,
      pub1,
      "tcp://127.0.0.1:7000"
    );
    const id2 = new cothority.ServerIdentity(
      curve,
      pub2,
      "tcp://127.0.0.1:7001"
    );
    const aggregate = curve.point().add(pub1, pub2);
    const roster = new cothority.Roster(
      curve,
      [id1, id2],
      new Uint8Array([1, 2, 3])
    );
    expect(roster.aggregateKey().equal(aggregate)).to.be.true;
  });
});

describe("server identity", () => {
  it("correctly creates its point representation", () => {
    const randomPoint = curve.point().pick();
    const randomBuff = randomPoint.marshalBinary();
    const addr = "tls://127.0.0.1:8000";
    const si = new cothority.ServerIdentity(curve, randomPoint, addr);
  });
});

function fakeIdentity(addr) {
  return {
    id: new Uint8Array([1]),
    //public: curve.point().pick(),
    public: new Uint8Array([1, 2, 3]),
    address: addr,
    description: "fake"
  };
}
