import fs from "fs";
import crypto from "crypto";
import BN = require('bn.js');
import Curve from '../../../src/curve/edwards25519/curve';
import { unhexlify, hexToBuffer } from "../../helpers/utils";

/**
 * Test vectors from http://ed25519.cr.yp.to/python/sign.input
 */
describe("Ed25519 Test Vector", () => {
    const curve = new Curve();

    let lines;
    beforeAll(done => {
        fs.readFile(__dirname + "/sign.input", "utf-8", (err, data) => {
            lines = data.split("\n");
            done();
        });
    });

    function testFactory(i) {
        it("vector " + i, () => {
            const parts = lines[i].split(":");
            const hash = crypto.createHash("sha512");
            let digest = hash.update(unhexlify(parts[0].substring(0, 64))).digest();
            digest = digest.slice(0, 32);
            digest[0] &= 0xf8;
            digest[31] &= 0x3f;
            digest[31] |= 0x40;
            const sk = new BN(digest.slice(0, 32), 16, "le");
            // using hexToBuffer until
            // https://github.com/indutny/bn.js/issues/175 is resolved
            const pk = new BN(hexToBuffer(parts[1]), 16, "le");
            const s = curve.scalar();
            s.unmarshalBinary(Buffer.from(sk.toArray("le")));
            const p = curve.point();
            p.unmarshalBinary(Buffer.from(pk.toArray("le")));

            const target = curve.point().mul(s);

            expect(p.equals(target)).toBeTruthy();
        });
    }

    for (let i = 0; i < 1024; i++) {
        testFactory(i);
    }
});
