const chai = require("chai");
const expect = chai.expect;

const mock = require('mock-socket');

const root = require("../../lib/protobuf").root;
const network = require("../../lib/net");

describe("sockets", () =>  {
    const serverAddr = "ws://127.0.0.1:8000";
    const deviceProtoName = "Device";
    const idProtoName = "ID";
    const message = new Uint8Array([1,2,3,4]);
    const deviceMessage = {
        point: message,
    }


    it("sends and receives correct protobuf messages",(done) => {
        const mockServer = new mock.Server(serverAddr+"/Device");
        mockServer.on('message', event => {
            const idProto = root.lookup(idProtoName);
            const id = {
                id: message,
            };
            const idMessage = idProto.create(id);
            const marshalled = idProto.encode(idMessage).finish();
            mockServer.send(marshalled);
        });

        const socket = new network.Socket(serverAddr);
        socket.send(deviceProtoName,idProtoName,deviceMessage).then((data) => {
            expect(data.id).to.deep.equal(deviceMessage.point);
            mockServer.stop(done);
        }).catch((err) => {
            expect(true,"socket send: " + err).to.be.false;
            mockServer.stop(done);
        });

    });
});



