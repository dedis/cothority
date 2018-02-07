const chai = require("chai");
const expect = chai.expect;

const misc = require("../../lib/misc");

describe("misc hex utilities", () => {

    const text = "43ef12ac1bc8";
    it("hextoUint8Array returns a Uint8Array buffer", () => {
        const buffer = misc.hexToUint8Array(text);
        expect(buffer).to.be.a("Uint8Array");
    });

    it("hex correctly decodes from a Uint8Array buffer", () => {
        const buffer = misc.hexToUint8Array(text)
        expect(buffer).to.be.a("Uint8Array");
        const expected = misc.uint8ArrayToHex(buffer);
        expect(expected).to.be.a("string");
        expect(expected).to.have.lengthOf(text.length);
        expect(expected).to.be.deep.equal(text);
    });
});

describe("misc buffer equality", () => {

    it("returns true for equal buffers", () => {
        const buffer1 = new Uint8Array([1,2,3,4]);
        const buffer2 = new Uint8Array([1,2,3,4]);
        expect(misc.uint8ArrayCompare(buffer1,buffer2)).to.be.true;
    });

    it("returns false for different buffers", () => {
        const buffer1 = new Uint8Array([1,2,3,4]);
        const buffer2 = new Uint8Array([1,2,3,3]);
        expect(misc.uint8ArrayCompare(buffer1,buffer2)).to.be.false;
    });

});

