import BN = require("bn.js");
import Nist from "../../../src/curve/nist";
import NistPoint from "../../../src/curve/nist/point";
// tslint:disable-next-line
const nistVectors = require("./ecdh_test.json");

const { Curve, Params, Point } = Nist;

describe("Nist vector tests", () => {
    const curve = new Curve(Params.p256);

    it("should work with NIST CAVP SP 800-56A ECCCDH Primitive Test Vectors", () => {
        // For each test vector calculate Z = privKey * peerPubKey and assert
        // X Coordinate of calcZ == Z
        for (const testVector of nistVectors) {
            const X = new BN(testVector.X, 16);
            const Y = new BN(testVector.Y, 16);
            const privKey = new BN(testVector.Private, 16);
            const peerX = new BN(testVector.PeerX, 16);
            const peerY = new BN(testVector.PeerY, 16);
            const Z = Buffer.from(new BN(testVector.Z, 16).toArray());

            const key = curve.scalar().setBytes(Buffer.from(privKey.toArray()));
            const pubKey = new Point(curve, X, Y);
            const peerPubKey = new Point(curve, peerX, peerY);

            const calcZ = curve.point().mul(key, peerPubKey) as NistPoint;

            expect(curve.curve.validate(pubKey.ref.point)).toBeTruthy();
            expect(curve.curve.validate(peerPubKey.ref.point)).toBeTruthy();
            expect(Z).toEqual(Buffer.from(calcZ.ref.point.getX().toArray()));
        }
    });
});
