import { BN256G2Point } from '../../src/pairing/point';
import Mask from '../../src/sign/mask';

describe('Mask Tests', () => {
    it('should create the correct aggregation', () => {
        const mask = Buffer.from([0b01010101, 0b00000001]);
        const publics = [];

        expect(() => new Mask(publics, mask)).toThrow();

        let agg = new BN256G2Point().null();
        for (let i = 0; i < 8; i++) {
            publics.push(new BN256G2Point().pick());
            expect(() => new Mask(publics, mask)).toThrow();

            if (i % 2 === 0) {
                agg = agg.add(agg, publics[i]);
            } else {
                agg = agg.add(agg, new BN256G2Point().neg(publics[i]));
            }
        }

        publics.push(new BN256G2Point().pick());
        agg = agg.add(agg, publics[8]);

        const m = new Mask(publics, mask);
        expect(m.aggregate.equals(agg)).toBeTruthy();
        expect(m.isIndexEnabled(0)).toBeTruthy();
        expect(m.isIndexEnabled(1)).toBeFalsy();
        expect(m.isIndexEnabled(8)).toBeTruthy();
    });

    function testMaskCount(mask: Mask, total: number, enabled: number): void {
        expect(mask.getCountTotal()).toBe(total);
        expect(mask.getCountEnabled()).toBe(enabled);
    }

    it('should return correct counts', () => {
        const publics = [];
        for (let i = 0; i < 16; i++) {
            publics.push(new BN256G2Point());
        }

        const vectors: [Mask, number, number][] = [
            [new Mask(publics.slice(0, 1), Buffer.from([0])), 1, 0],
            [new Mask(publics.slice(0, 5), Buffer.from([0b10101])), 5, 3],
            [new Mask(publics, Buffer.from([0b11111111, 0b11])), 16, 10],
        ];

        for (let i = 0; i < vectors.length; i++) {
            testMaskCount(vectors[i][0], vectors[i][1], vectors[i][2])
        }
    })
});
