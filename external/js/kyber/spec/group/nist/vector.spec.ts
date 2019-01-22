import BN = require('bn.js');
import Nist from '../../../src/curve/nist';
import NistPoint from '../../../src/curve/nist/point';
const nistVectors = require("./ecdh_test.json");

const { Curve, Params, Point } = Nist;

describe("Nist vector tests", () => {
    const curve = new Curve(Params.p256);

    it("should work with NIST CAVP SP 800-56A ECCCDH Primitive Test Vectors", () => {
        // For each test vector calculate Z = privKey * peerPubKey and assert
        // X Coordinate of calcZ == Z
        for (let i = 0; i < nistVectors.length; i++) {
            let testVector = nistVectors[i];
            let X = new BN(testVector.X, 16);
            let Y = new BN(testVector.Y, 16);
            let privKey = new BN(testVector.Private, 16);
            let peerX = new BN(testVector.PeerX, 16);
            let peerY = new BN(testVector.PeerY, 16);
            let Z = Buffer.from(new BN(testVector.Z, 16).toArray());

            let key = curve.scalar().setBytes(Buffer.from(privKey.toArray()));
            let pubKey = new Point(curve, X, Y);
            let peerPubKey = new Point(curve, peerX, peerY);

            let calcZ = curve.point().mul(key, peerPubKey) as NistPoint;

            expect(curve.curve.validate(pubKey.ref.point)).toBeTruthy();
            expect(curve.curve.validate(peerPubKey.ref.point)).toBeTruthy();
            expect(Z).toEqual(Buffer.from(calcZ.ref.point.getX().toArray()));
        }
    });
});