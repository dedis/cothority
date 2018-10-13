const chai = require("chai");
const expect = chai.expect;

const root = require("../../lib/protobuf").root;
const network = require("../../lib/net");
const identity = require("../../lib/identity.js");

const helpers = require("../helpers.js");
const co = require("co");
const WebSocket = require("ws");

const kyber = require("@dedis/kyber-js");
const ed25519 = new kyber.curve.edwards25519.Curve();

describe("leader socket", () => {
  it("fails with no node", () => {
    const roster = new identity.Roster(ed25519, []);
    expect(function() {
      const socket = new network.LeaderSocket(roster, "cisc");
    }).to.throw("Roster should have at least one node");
  });

  it("uses the first node in the roster", done => {
    function createMockServerForLeader(port) {
      const mockServer = new WebSocket.Server({
        host: "127.0.0.1",
        port: port
      });

      mockServer.on("connection", function connection(ws) {
        ws.on("message", function incoming() {
          const proto = root.lookup("ServerIdentity");
          const ret = {
            id: [],
            public: [],
            address: `tcp://127.0.0.1:${port - 1}`,
            description: ""
          };
          const marshalled = proto.encode(ret).finish();
          ws.send(marshalled);
        });
      });
      return mockServer;
    }

    const n = 5;
    // create the addresses
    const identities = [];
    let server;
    for (let i = 0; i < n; i++) {
      identities[i] = new identity.ServerIdentity(
        ed25519,
        ed25519.point().pick(),
        "tcp://127.0.0.1:600" + i
      );
      if (i == 0) {
        wsAddr = identities[i].websocketAddr + "/test/identity";
        server = createMockServerForLeader("6001");
      }
    }
    const roster = new identity.Roster(ed25519, identities);
    // create the socket and see if we have any messages back
    const socket = new network.LeaderSocket(roster, "test");
    socket
      .send("status.Request", "ServerIdentity", {})
      .then(data => {
        expect(data.address).to.equal("tcp://127.0.0.1:6000");
        server.close(done);
      })
      .catch(err => {
        server.close(done);
        throw err;
      });
  });

  it("retries 3 times in case of an error", done => {
    const n = 5;
    // create the addresses
    const identities = [];
    for (let i = 0; i < n; i++) {
      identities[i] = new identity.ServerIdentity(
        ed25519,
        ed25519.point().pick(),
        "tcp://127.0.0.1:600" + i
      );
      if (i == 0) {
        wsAddr = identities[i].websocketAddr + "/test/identity";
      }
    }
    const roster = new identity.Roster(ed25519, identities);
    // create the socket and see if we have any messages back
    const socket = new network.LeaderSocket(roster, "test");
    socket.send("status.Request", "ServerIdentity", {}).catch(e => {
      expect(e.message).to.include("couldn't send request after 3 attempts");
      done();
    });
  });
});

describe("real server status", () => {
  var proc;
  after(function() {
    helpers.killGolang(proc);
  });

  it("can talk to status", done => {
    const build_dir = process.cwd() + "/test/skipchain/build";

    var fn = co.wrap(function*() {
      [roster, id] = helpers.readSkipchainInfo(build_dir);
      const socket = new network.RosterSocket(roster, "Status");
      socket
        .send("status.Request", "Response", {})
        .then(data => {
          expect(data.status.Db.field.Open).to.equal("true");
          done();
        })
        .catch(err => {
          throw err;
          done();
        });
    });

    helpers
      .runGolang(build_dir, data => data.match(/OK/))
      .then(proces => {
        proc = proces;
        return Promise.resolve(true);
      })
      .then(fn)
      .catch(err => {
        done();
        throw err;
      });
  }).timeout(5000);
});
