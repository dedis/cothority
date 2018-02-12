const chai = require("chai");
const expect = chai.expect;

const root = require("../../lib/protobuf").root;
const network = require("../../lib/net");
const cothority = require("../../lib");
const identity = require("../../lib/identity.js");

const kyber = require("@dedis/kyber-js");
const ed25519 = new kyber.curve.edwards25519.Curve();
const serverAddr = "ws://127.0.0.1:9000";
const deviceProtoName = "Device";
const idProtoName = "ID";
const message = new Uint8Array([1, 2, 3, 4]);
const deviceMessage = {
  point: message
};

//const mock = require("mock-socket");
const WebSocket = require("ws");
describe("sockets", () => {
  it("sends and receives correct protobuf messages", done => {
    const mockServer = createServer("9000");
    /*const mockServer = new mock.Server(serverAddr + "/cisc/Device");*/
    //mockServer.on("message", event => {
    //const idProto = root.lookup(idProtoName);
    //const id = {
    //id: message
    //};
    //const idMessage = idProto.create(id);
    //const marshalled = idProto.encode(idMessage).finish();
    //mockServer.send(marshalled);
    //});

    const socket = new network.Socket(serverAddr, "cisc");
    socket
      .send(deviceProtoName, idProtoName, deviceMessage)
      .then(data => {
        expect(data.id).to.deep.equal(deviceMessage.point);
        mockServer.close(done);
      })
      .catch(err => {
        //expect(true, "socket send: " + err).to.be.false;
        mockServer.close(done);
        throw err;
      });
  });
});

function createServer(port) {
  const mockServer = new WebSocket.Server({
    host: "127.0.0.1",
    port: port
  });

  mockServer.on("connection", function connection(ws) {
    ws.on("message", function incoming(msg) {
      console.log("received: ", msg.toString());
      const idProto = root.lookup(idProtoName);
      const id = {
        id: message
      };
      const idMessage = idProto.create(id);
      const marshalled = idProto.encode(idMessage).finish();
      ws.send(marshalled);
    });
  });
  return mockServer;
}

describe.only("roster socket", () => {
  it("tries all servers", done => {
    const n = 5;
    // create the addresses
    const identities = [];
    var server = "";
    for (var i = 0; i < n; i++) {
      identities[i] = new identity.ServerIdentity(
        ed25519,
        ed25519.point().pick(),
        "tcp://127.0.0.1:700" + i
      );
      if (i == n - 1) {
        wsAddr = identities[i].websocketAddr + "/cisc/Device";
        server = createServer("700" + i);
      }
    }
    const roster = new identity.Roster(identities);
    // create the socket and see if we have any messages back
    const socket = new network.RosterSocket(roster, "cisc");
    socket
      .send(deviceProtoName, idProtoName, deviceMessage)
      .then(data => {
        expect(data.id).to.deep.equal(deviceMessage.point);
        server.close(done);
      })
      .catch(err => {
        //expect(true, "socket send: " + err).to.be.false;
        server.close(done);
        throw err;
      });
  });
});
