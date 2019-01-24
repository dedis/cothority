import { zeroBN } from '../src/constants'

describe('Kyber constants', () => {
    it('should provide the 0 constant using big number', () => {
        expect(zeroBN).toBeDefined();
    });
});
