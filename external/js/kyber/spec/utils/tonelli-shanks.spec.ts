import BN from 'bn.js';
import { modSqrt } from '../../src/utils/tonelli-shanks';

describe('Tonelli-Shanks', () => {
    it('should pass the reference test', () => {
        expect(modSqrt(10, 13).toNumber()).toBe(7);
        expect(modSqrt(56, 101).toNumber()).toBe(37);
        expect(modSqrt(1030, 10009).toNumber()).toBe(1632);
        expect(modSqrt(665820697, 1000000009).toNumber()).toBe(378633312);
        expect(modSqrt(1032, 10009)).toBeNull();

        const a = "41660815127637347468140745042827704103445750172002";
        const b = "100000000000000000000000000000000000000000000000577";
        expect(modSqrt(a, b).eq(new BN("32102985369940620849741983987300038903725266634508"))).toBeTruthy();
    });
});
