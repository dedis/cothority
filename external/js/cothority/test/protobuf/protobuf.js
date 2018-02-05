const mocha = require("mocha");
const chai = require("chai");
const expect = chai.expect;

const proto = require("../../lib/protobuf");
const root = proto.root;

describe("protobuf", () =>  {
    it("correctly marshals and unmarshals a protobuf structure", () =>  {
        const messageType = "Device";
        const buff = new Uint8Array([1,2,3,4]);
        const device =  {
            point: buff,
        }
        const deviceProto = root.lookup(messageType);
        expect(deviceProto.verify(device)).to.be.null;
        const message = deviceProto.create(device);
        const marshalled = deviceProto.encode(device).finish();

        const deviceProto2 = root.lookup(messageType);
        const unmarshalled = deviceProto2.decode(marshalled);

        expect(device.point).to.deep.equal(unmarshalled.point);
    });
});
