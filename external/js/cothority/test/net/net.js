const chai = require("chai");
const expect = chai.expect;

const mock = require('mock-socket');
const root = require("../../lib/protobuf").root;
const network = require("../../lib/net");
const cothority = require("../../lib");

const serverAddr = "ws://127.0.0.1:9000";
const deviceProtoName = "Device";
const idProtoName = "ID";
const message = new Uint8Array([1,2,3,4]);
const deviceMessage = {
    point: message,
}

describe("sockets", () =>  {

    it("sends and receives correct protobuf messages",(done) => {
        const mockServer = new mock.Server(serverAddr+"/cisc/Device");
        mockServer.on('message', event => {
            const idProto = root.lookup(idProtoName);
            const id = {
                id: message, 
            };
            const idMessage = idProto.create(id);
            const marshalled = idProto.encode(idMessage).finish();
            mockServer.send(marshalled);
        });

        const socket = new network.Socket(serverAddr,"cisc");
        socket.send(deviceProtoName,idProtoName,deviceMessage).then((data) => {
            expect(data.id).to.deep.equal(deviceMessage.point);
            mockServer.stop(done);
        }).catch((err) => {
            expect(true,"socket send: " + err).to.be.false;
            mockServer.stop(done);
        });
    });
});

describe("roster socket", () => {

    const createServer = function(addr) {
        const mockServer = new mock.Server(addr);
        mockServer.on('message', event => {
            const idProto = root.lookup(idProtoName);
            const id = {
                id: message, 
            };
            const idMessage = idProto.create(id);
            const marshalled = idProto.encode(idMessage).finish();
            mockServer.send(marshalled);
        });
        return mockServer;
    }
    
    it("tries all servers", (done) => {
        const n = 5;
        // create the addresses
        const addresses = [];
        var server = "";
        for (var i = 0; i < n; i++) {
            addresses[i] = "ws://127.0.0.1:700" + i;
            if (i == n-1) {
                wsAddr = addresses[i] + "/cisc/Device";
                server = createServer(wsAddr);
            }
        }
        // create the socket and see if we have any messages back 
        const socket = new network.RosterSocket(addresses,"cisc");
        socket.send(deviceProtoName,idProtoName,deviceMessage).then((data) => {
            expect(data.id).to.deep.equal(deviceMessage.point);
            server.stop(done);
        }).catch((err) => {
            expect(true,"socket send: " + err).to.be.false;
            server.stop(done);
        });
       
    });

});
