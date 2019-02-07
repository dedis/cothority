import Mask from '../../src/sign/mask';
import { BN256G2Point } from '../../src/pairing/point';

describe('Mask Tests', () => {
    it('should create the correct aggregation', () => {
        const mask = Buffer.from([0b01010101, 0b00000001]);
        const publics = [];

        expect(() => new Mask(publics, mask)).toThrow();

        let agg = new BN256G2Point().null();
        for (let i = 0; i < 8; i++) {
            publics.push(new BN256G2Point().pick());
            expect(() => new Mask(publics, mask)).toThrow();

            if (i%2 === 0) {
                agg = agg.add(agg, publics[i]);
            } else {
                agg = agg.add(agg, new BN256G2Point().neg(publics[i]));
            }
        }

        publics.push(new BN256G2Point().pick());
        agg = agg.add(agg, publics[8]);

        const m = new Mask(publics, mask);
        expect(m.aggregate.equals(agg)).toBeTruthy();
    });
});
