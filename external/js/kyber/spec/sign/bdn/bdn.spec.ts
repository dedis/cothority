import { BN256G2Point } from "../../../src/pairing/point";
import BN256Scalar from "../../../src/pairing/scalar";
import { hashPointToR } from "../../../src/sign/bdn";

describe('BDN signatures Test', () => {
    it('should hash to R', () => {
        const p1 = new BN256G2Point().base();
        const p2 = new BN256G2Point().mul(new BN256Scalar(2), p1);
        const p3 = new BN256G2Point().mul(new BN256Scalar(3), p1);

        const coefs = hashPointToR([p1, p2, p3]);

        expect(coefs[0].toString('hex')).toBe('35b5b395f58aba3b192fb7e1e5f2abd3');
        expect(coefs[1].toString('hex')).toBe('14dcc79d46b09b93075266e47cd4b19e');
        expect(coefs[2].toString('hex')).toBe('933f6013eb3f654f9489d6d45ad04eaf');
    });
});
