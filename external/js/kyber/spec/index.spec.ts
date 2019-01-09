import { sign, curve } from '../src';

describe('Kyber', () => {
    it('should provide the curves', () => {
        expect(curve).toBeDefined();
    });

    it('should provide the signatures', () => {
        expect(sign).toBeDefined();
    });
});
