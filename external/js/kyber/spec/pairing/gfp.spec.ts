import BN = require('bn.js');
import GfP from '../../src/pairing/gfp';

describe('GfP', () => {
    it('should compute the modulo', () => {
        const v = new GfP(-3);

        expect(v.mod(new BN(5)).getValue().toNumber()).toBe(2);
    });
});
